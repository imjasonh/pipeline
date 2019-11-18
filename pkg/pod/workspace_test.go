package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	testnames "github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestSetWorkspaceAndHome(t *testing.T) {
	testnames.TestingSeed()
	got, _, vms := setWorkspaceAndHome([]v1alpha1.Step{{Container: corev1.Container{
		Image: "step-1",
	}}, {Container: corev1.Container{
		Image:      "step-2",
		WorkingDir: "/do/not/overwrite",
		Env: []corev1.EnvVar{{
			Name:  "HOME",
			Value: "sweet-home",
		}},
	}}, {
		Script: "my-script",
		Container: corev1.Container{
			Image: "step-3",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "my-own-workspace",
				MountPath: "/workspace",
			}},
		},
	}})

	want := []v1alpha1.Step{{Container: corev1.Container{
		Image:        "step-1",
		WorkingDir:   workspaceDir,
		Env:          implicitEnvVars,
		VolumeMounts: vms,
	}}, {Container: corev1.Container{
		Image:      "step-2",
		WorkingDir: "/do/not/overwrite",
		Env: append(implicitEnvVars, []corev1.EnvVar{{
			Name:  "HOME",
			Value: "sweet-home",
		}}...),
		VolumeMounts: vms,
	}}, {
		Script: "my-script",
		Container: corev1.Container{
			Image:      "step-3",
			WorkingDir: workspaceDir,
			Env:        implicitEnvVars,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "my-own-workspace",
				MountPath: "/workspace",
			}, {
				Name:      "home-mz4c7",
				MountPath: "/builder/home",
			}},
		},
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff (-want, +got): %s", d)
	}
}
