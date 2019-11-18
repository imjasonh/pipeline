package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
	return x.Cmp(y) == 0
})

func TestSetResources(t *testing.T) {
	for _, c := range []struct {
		desc     string
		in, want []corev1.Container
	}{{
		desc: "max across steps",
		in: []corev1.Container{{
			Image: "step-1",
			// No resources requested.
		}, {
			Image: "step-2",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
			},
		}, {
			Image: "step-3",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("1.9Gi"),
					corev1.ResourceEphemeralStorage: zeroQty,
				},
			},
		}},
		want: []corev1.Container{{
			Image: "step-1",
			// First step has maximum of all types.
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("1.9Gi"),
					// Ephemeral storage is not specified, because nothing has specified any request.
				},
			},
		}, {
			// All other steps have zero resources requested.
			Image: "step-2",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              zeroQty,
					corev1.ResourceMemory:           zeroQty,
					corev1.ResourceEphemeralStorage: zeroQty,
				},
			},
		}, {
			Image: "step-3",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              zeroQty,
					corev1.ResourceMemory:           zeroQty,
					corev1.ResourceEphemeralStorage: zeroQty,
				},
			},
		}},
	}, {
		desc: "one step",
		in: []corev1.Container{{
			Image: "step",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
			},
		}},
		want: []corev1.Container{{
			Image: "step",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
			},
		}},
	}, {
		desc: "nothing requested",
		in: []corev1.Container{{
			Image: "step-1",
		}, {
			Image: "step-2",
		}},
		want: []corev1.Container{{
			Image:     "step-1",
			Resources: corev1.ResourceRequirements{},
		}, {
			Image:     "step-2",
			Resources: corev1.ResourceRequirements{},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := setResources(c.in)
			if d := cmp.Diff(c.want, got, resourceQuantityCmp); d != "" {
				t.Fatalf("Diff (-want, +got): %s", d)
			}
		})
	}
}
