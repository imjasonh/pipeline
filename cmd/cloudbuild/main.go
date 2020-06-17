package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	runinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/run"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	cloudbuild "google.golang.org/api/cloudbuild/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	controllerName = "cloudbuild-task-controller"
	pollDuration   = 2 * time.Second
)

func main() {
	sharedmain.Main(controllerName, newController)
}

func newController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	projectID, err := metadata.ProjectID()
	if err != nil {
		logger.Fatalf("Getting project ID from metadata: %v", err)
	}
	logger.Infof("Will create builds in project %q", projectID)

	svc, err := cloudbuild.NewService(ctx)
	if err != nil {
		logger.Fatalf("Setting up Cloud Build service: %v", err)
	}

	if _, err := svc.Projects.Builds.List(projectID).Do(); err != nil {
		logger.Fatalf("Failed to list builds in preflight check: %v", err)
	}
	logger.Info("Preflight check passed!")

	c := &Reconciler{
		gcb:       svc,
		projectID: projectID,
	}
	impl := runreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: controllerName,
		}
	})
	c.enqueueAfter = impl.EnqueueAfter

	runinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: FilterRunRef("cloudbuild.googleapis.com/v1alpha1", "Build"),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	return impl
}

// FilterRunRef returns a filter that can be passed to a Run Informer, which
// filters out Runs for apiVersion and kinds that this controller doesn't care
// about.
// TODO: Provide this as a helper function.
func FilterRunRef(apiVersion, kind string) func(interface{}) bool {
	return func(obj interface{}) bool {
		r, ok := obj.(*v1alpha1.Run)
		if !ok {
			// Somehow got informed of a non-Run object.
			// Ignore.
			return false
		}
		if r == nil || r.Spec.Ref == nil {
			// These are invalid, but just in case they get
			// created somehow, don't panic.
			return false
		}

		return r.Spec.Ref.APIVersion == apiVersion && r.Spec.Ref.Kind == v1alpha1.TaskKind(kind)
	}
}

type Reconciler struct {
	gcb       *cloudbuild.Service
	projectID string

	enqueueAfter func(interface{}, time.Duration)
}

// ReconcileKind implements Interface.ReconcileKind.
func (c *Reconciler) ReconcileKind(ctx context.Context, r *v1alpha1.Run) reconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling %s/%s", r.Namespace, r.Name)

	if r.IsDone() {
		logger.Info("Run is finished, done reconciling")
		return nil
	}

	var b *cloudbuild.Build

	cond := r.Status.Status.GetCondition(apis.ConditionSucceeded)
	if cond == nil {
		// Build hasn't started, so let's start one.
		logger.Infof("Run hasn't started, starting Build...")
		// TODO: Don't hard-code the build, look up the ref and
		// interpret that as a Build.

		op, err := c.gcb.Projects.Builds.Create(c.projectID, &cloudbuild.Build{
			Steps: []*cloudbuild.BuildStep{{
				Name:       "ubuntu",
				Entrypoint: "bash",
				Args:       []string{"-c", "sleep 5"},
			}, {
				Name:       "ubuntu",
				Entrypoint: "bash",
				Args:       []string{"-c", "sleep 5"},
			}},
		}).Do()
		if err != nil {
			logger.Errorf("Error creating Build: %v", err)
			return err
		}

		var bomd cloudbuild.BuildOperationMetadata
		if err := json.Unmarshal(op.Metadata, &bomd); err != nil {
			logger.Errorf("Error unmarshaling operation metadata: %v", err)
			return err
		}
		b = bomd.Build
		if b == nil {
			logger.Error("Operation had nil build metadata")
			return errors.New("operation had nil build metadata")
		}

		logger.Infof("Created build %q", b.Id)
	} else {
		logger.Infof("Run has started (%s), checking build status...", cond.Reason)
		// The Run has started, so check on the status of the build
		// and update the Run's status with the latest details.
		id, err := r.Status.GetAdditionalField("buildId")
		if err != nil {
			return err
		}
		buildID, ok := id.(string)
		if !ok {
			return fmt.Errorf("build ID wasn't a string: %T", id)
		}
		logger.Infof("Getting build %q", buildID)

		b, err = c.gcb.Projects.Builds.Get(c.projectID, buildID).Do()
		if err != nil {
			logger.Errorf("Getting Build %q: %v", buildID, err)
			return err
		}
	}

	logger.Infof("Build %q status is %q", b.Id, b.Status)
	r.Status.Status.SetConditions([]apis.Condition{conditions[b.Status]})
	r.Status.StartTime = parseTime(b.StartTime)
	r.Status.CompletionTime = parseTime(b.FinishTime)

	// Update the build metadata in the Run status.
	if err := r.Status.SetAdditionalField("buildId", b.Id); err != nil {
		logger.Errorf("Error setting build ID metadata: %v", err)
		return err
	}
	if err := r.Status.SetAdditionalField("build", b); err != nil {
		logger.Errorf("Error setting build metadata: %v", err)
		return err
	}

	// If the build isn't done, enqueue another reconcile of this
	// Run at some point in the future.
	if !r.IsDone() {
		logger.Infof("Build is not done, will check again in %s", pollDuration)
		c.enqueueAfter(r, pollDuration)
	}

	return reconciler.NewEvent(corev1.EventTypeNormal, "RunReconciled", "Run reconciled: \"%s/%s\"", r.Namespace, r.Name)
}

func parseTime(s string) *metav1.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Printf("Parsing %q: %v", s, err)
		return nil
	}
	mt := metav1.NewTime(t)
	return &mt
}

var conditions = map[string]apis.Condition{
	"STATUS_UNKNOWN": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "UnknownStatus",
		Message: "The build's status is unknown",
	},
	"QUEUED": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "BuildQueued",
		Message: "The build is queued",
	},
	"WORKING": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Reason:  "BuildWorking",
		Message: "The build is currently running",
	},
	"SUCCESS": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionTrue,
		Reason:  "BuildSucceeded",
		Message: "The build has succeeded",
	},
	"FAILURE": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "BuildFailed",
		Message: "The build has failed",
	},
	"INTERNAL_ERROR": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "InternalError",
		Message: "The build encountered an internal error",
	},
	"TIMEOUT": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "BuildTimeout",
		Message: "The build exceeded its configured timeout",
	},
	"CANCELLED": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "BuildCancelled",
		Message: "The build was cancelled",
	},
	"EXPIRED": apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionFalse,
		Reason:  "BuildExpired",
		Message: "The build was enqueued for longer than the value of its queue TTL",
	},
}
