/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"

	defaultconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/contexts"
	"github.com/tektoncd/pipeline/pkg/system"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	pkgleaderelection "knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// v1alpha1
	v1alpha1.SchemeGroupVersion.WithKind("Pipeline"):         &v1alpha1.Pipeline{},
	v1alpha1.SchemeGroupVersion.WithKind("Task"):             &v1alpha1.Task{},
	v1alpha1.SchemeGroupVersion.WithKind("ClusterTask"):      &v1alpha1.ClusterTask{},
	v1alpha1.SchemeGroupVersion.WithKind("TaskRun"):          &v1alpha1.TaskRun{},
	v1alpha1.SchemeGroupVersion.WithKind("PipelineRun"):      &v1alpha1.PipelineRun{},
	v1alpha1.SchemeGroupVersion.WithKind("Condition"):        &v1alpha1.Condition{},
	v1alpha1.SchemeGroupVersion.WithKind("PipelineResource"): &v1alpha1.PipelineResource{},
	// v1beta1
	v1beta1.SchemeGroupVersion.WithKind("Pipeline"):    &v1beta1.Pipeline{},
	v1beta1.SchemeGroupVersion.WithKind("Task"):        &v1beta1.Task{},
	v1beta1.SchemeGroupVersion.WithKind("ClusterTask"): &v1beta1.ClusterTask{},
	v1beta1.SchemeGroupVersion.WithKind("TaskRun"):     &v1beta1.TaskRun{},
	v1beta1.SchemeGroupVersion.WithKind("PipelineRun"): &v1beta1.PipelineRun{},
}

func newDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.pipeline.tekton.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return contexts.WithUpgradeViaDefaulting(store.ToContext(ctx))
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func newValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.pipeline.tekton.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return contexts.WithUpgradeViaDefaulting(store.ToContext(ctx))
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func newConfigValidationController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,

		// Name of the configmap webhook.
		"config.webhook.pipeline.tekton.dev",

		// The path on which to serve the webhook.
		"/config-validation",

		// The configmaps to validate.
		configmap.Constructors{
			logging.ConfigMapName():               logging.NewConfigFromConfigMap,
			defaultconfig.GetDefaultsConfigName(): defaultconfig.NewDefaultsFromConfigMap,
			pkgleaderelection.ConfigMapName():     pkgleaderelection.NewConfigFromConfigMap,
		},
	)
}

func newConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// nolint: golint
	var (
		v1alpha1GroupVersion = v1alpha1.SchemeGroupVersion.Version
		v1beta1GroupVersion  = v1beta1.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			v1beta1.Kind("Task"): {
				DefinitionName: pipeline.TaskResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.Task{},
					v1beta1GroupVersion:  &v1beta1.Task{},
				},
			},
			v1beta1.Kind("ClusterTask"): {
				DefinitionName: pipeline.ClusterTaskResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.ClusterTask{},
					v1beta1GroupVersion:  &v1beta1.ClusterTask{},
				},
			},
			v1beta1.Kind("TaskRun"): {
				DefinitionName: pipeline.TaskRunResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.TaskRun{},
					v1beta1GroupVersion:  &v1beta1.TaskRun{},
				},
			},
			v1beta1.Kind("Pipeline"): {
				DefinitionName: pipeline.PipelineResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.Pipeline{},
					v1beta1GroupVersion:  &v1beta1.Pipeline{},
				},
			},
			v1beta1.Kind("PipelineRun"): {
				DefinitionName: pipeline.PipelineRunResource.String(),
				HubVersion:     v1alpha1GroupVersion,
				Zygotes: map[string]conversion.ConvertibleObject{
					v1alpha1GroupVersion: &v1alpha1.PipelineRun{},
					v1beta1GroupVersion:  &v1beta1.PipelineRun{},
				},
			},
		},

		// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata
		func(ctx context.Context) context.Context {
			return ctx
		},
	)
}

func main() {
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "tekton-pipelines-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "webhook-certs" // #nosec
	}

	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), system.GetNamespace())

	// Set up a signal context with our webhook options
	ctx = webhook.WithOptions(ctx, webhook.Options{
		ServiceName: serviceName,
		Port:        8443,
		SecretName:  secretName,
	})

	sharedmain.WebhookMainWithConfig(ctx, "webhook",
		sharedmain.ParseAndGetConfigOrDie(),
		certificates.NewController,
		newDefaultingAdmissionController,
		newValidationAdmissionController,
		newConfigValidationController,
		newConversionController,
		newStateValidator,
	)
}

func newStateValidator(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	logger := logging.FromContext(ctx)

	sv := stateValidator{}
	return controller.NewImpl(sv, logger, "work-queue-name")
}

type stateValidator struct {
	webhook.StatelessAdmissionController
}

func (stateValidator) Path() string { return "/run-state-validator" }

func (stateValidator) Reconcile(ctx context.Context, key string) error { return nil }

var (
	runGVK = metav1.GroupVersionKind{
		Group:   "tekton.dev",
		Version: "v1alpha1",
		Kind:    "Run",
	}
	allowed = &admissionv1.AdmissionResponse{Allowed: true}
)

func (stateValidator) Admit(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// Ignore non-Run objects.
	if req.Kind != runGVK {
		return allowed
	}

	// Ignore creations and deletions.
	if req.Operation == admissionv1.Create ||
		req.Operation == admissionv1.Delete {
		return allowed
	}

	// Decode old and new objects.
	var oldObj, newObj v1alpha1.Run
	if err := json.NewDecoder(bytes.NewReader(req.Object.Raw)).Decode(&newObj); err != nil {
		return webhook.MakeErrorStatus("validation failed: cannot decode incoming new object: %v", err)
	}
	if err := json.NewDecoder(bytes.NewReader(req.OldObject.Raw)).Decode(&oldObj); err != nil {
		return webhook.MakeErrorStatus("validation failed: cannot decode incoming old object: %v", err)
	}

	log.Println("====== VALIDATING =====")
	if err := v1alpha1.ValidateRunTransition(oldObj, newObj); err != nil {
		log.Println("--> invalid!")
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}
	log.Println("--> valid!")
	return allowed
}
