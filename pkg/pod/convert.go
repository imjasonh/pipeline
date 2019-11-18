package pod

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func MakePod(sidecars []corev1.Container, steps []v1alpha1.Step) ([]corev1.Container, error) {
	var epCache EntrypointCache             // TODO
	namespace, serviceAccountName := "", "" // TODO
	var stepTemplate *corev1.Container      // TODO

	// Merge StepTemplate with all steps.
	steps, err := mergeStepTemplate(stepTemplate, steps)
	if err != nil {
		return nil, err
	}

	// TODO Run creds-init as init container
	// TODO Prepend+append resource containers

	// Attach /workspace and /builder/home to all steps, and set HOME env var.
	steps, _, _ = setWorkspaceAndHome(steps) // TODO use returns

	// Convert any steps with Scripts to regular Containers.
	stepContainers, _, _ := convertScripts(shellImage, steps) // TODO use returns

	// Resolve any image entrypoints.
	// After this, all Containers have a Command specified.
	sidecars, err = resolveEntrypoints(epCache, namespace, serviceAccountName, sidecars)
	if err != nil {
		return nil, err
	}
	stepContainers, err = resolveEntrypoints(epCache, namespace, serviceAccountName, stepContainers)
	if err != nil {
		return nil, err
	}

	// Set resource requests so only the max resource requests are
	// considered, not the sum of all steps' requests.
	//
	// We don't do this for sidecar containers, because they will all be
	// running at the same time, so their requests should be summed.
	stepContainers = setResources(stepContainers)

	// Redirect entrypoints to order containers.
	containers := orderContainers(sidecars, stepContainers)
	// TODO: Make the whole pod.
	return containers, nil
}
