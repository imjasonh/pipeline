package pod

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	zeroQty       = resource.MustParse("0")
	resourceNames = []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage}
)

// setResources sets CPU/memory/storage resource requests for all step
// containers.
//
// Because steps are run in order, the pod doesn't need to request the *sum* of
// all step containers' resources, we only need one container to request the
// *maximum* requests for all available resource types. We choose to assign the
// first step that maximum request.
func setResources(orig []corev1.Container) []corev1.Container {
	steps := orig[:] // copy
	if len(steps) == 1 {
		return steps
	}

	max := map[corev1.ResourceName]resource.Quantity{}
	for _, s := range steps {
		for _, n := range resourceNames {
			currentMax := max[n]
			if req, found := s.Resources.Requests[n]; found && req.Cmp(currentMax) > 0 {
				max[n] = req
			}
		}
	}
	// Nothing was requested, we can just skip the rest.
	if len(max) == 0 {
		return steps
	}

	// Set the first container's resources to the max, and zero out all
	// others.
	// TODO
	for n, r := range max {
		if steps[0].Resources.Requests == nil {
			steps[0].Resources.Requests = corev1.ResourceList{}
		}
		steps[0].Resources.Requests[n] = r
	}
	for i := 1; i < len(steps); i++ {
		if steps[i].Resources.Requests == nil {
			steps[i].Resources.Requests = corev1.ResourceList{}
		}
		for _, n := range resourceNames {
			steps[i].Resources.Requests[n] = zeroQty
		}
	}
	return steps
}
