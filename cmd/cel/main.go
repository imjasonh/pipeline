package main

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	runinformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1alpha1/run"
	runreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1alpha1/run"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	controllerName = "cel-task-controller"
)

func main() {
	sharedmain.Main(controllerName, newController)
}

func newController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	c := &Reconciler{}
	impl := runreconciler.NewImpl(ctx, c, func(impl *controller.Impl) controller.Options {
		return controller.Options{
			AgentName: controllerName,
		}
	})

	runinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: FilterRunRef("cel.example.dev/v0", "CEL"),
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
}

// ReconcileKind implements Interface.ReconcileKind.
func (c *Reconciler) ReconcileKind(ctx context.Context, r *v1alpha1.Run) reconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling %s/%s", r.Namespace, r.Name)

	expr := r.Spec.GetParam("expression")
	if expr == nil || expr.Value.StringVal == "" {
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "MissingExpression",
			Message: "The expression param was not passed",
		}})
		return nil
	}

	// Evaluate the CEL expression.
	env, err := cel.NewEnv(cel.Declarations())
	if err != nil {
		logger.Errorf("cel.NewEnv: %v", err)
		return err
	}
	ast, iss := env.Compile(expr.Value.StringVal)
	if iss.Err() != nil {
		// Syntax error.
		// TODO: report error string.
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "SyntaxError",
			Message: "The expression could not be parsed",
		}})
		return nil
	}

	prg, err := env.Program(ast)
	out, _, err := prg.Eval(map[string]interface{}{})
	if err != nil {
		// Error evaluating.
		// TODO: report error string.
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "EvaluationError",
			Message: "The expression could not be evaluated",
		}})
		return nil
	}

	if !types.IsBool(out) {
		logger.Errorf("result (%v) is not boolean (type: %s)", out.Value(), out.Type().TypeName())
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "NonBoolValue",
			Message: "The expression value was not a boolean",
		}})
		return err
	}

	if out == types.Bool(true) {
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  "ExpressionTrue",
			Message: "The expression evaluated to true",
		}})
	} else {
		r.Status.Status.SetConditions([]apis.Condition{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Reason:  "ExpressionFalse",
			Message: "The expression evaluated to false",
		}})
	}

	return reconciler.NewEvent(corev1.EventTypeNormal, "RunReconciled", "Run reconciled: \"%s/%s\"", r.Namespace, r.Name)
}
