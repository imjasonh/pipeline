package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

var volumeMount = corev1.VolumeMount{
	Name:      "my-mount",
	MountPath: "/mount/point",
}

func TestOrderedContainers(t *testing.T) {
	sidecars := []corev1.Container{{
		Image:   "sidecar-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "sidecar-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}}
	steps := []corev1.Container{{
		Image:   "step-1",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}, {
		Image:        "step-2",
		Command:      []string{"cmd1", "cmd2", "cmd3"}, // multiple cmd elements
		Args:         []string{"arg1", "arg2"},
		VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
	}, {
		Image:   "step-3",
		Command: []string{"cmd"},
		Args:    []string{"arg1", "arg2"},
	}}
	want := []corev1.Container{{
		Image:   "sidecar-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-start_file", "/builder/tools/sidecar-0",
			"-kill_file", "/builder/tools/sidecar-kill",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}, {
		Image:   "sidecar-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-start_file", "/builder/tools/sidecar-1",
			"-kill_file", "/builder/tools/sidecar-kill",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3", "arg1", "arg2",
		},
		VolumeMounts: []corev1.VolumeMount{volumeMount, toolsMount},
	}, {
		Image:   "step-1",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/builder/tools/sidecar-0,/builder/tools/sidecar-1",
			"-post_file", "/builder/tools/0",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}, {
		Image:   "step-2",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/builder/tools/0",
			"-post_file", "/builder/tools/1",
			"-entrypoint", "cmd1", "--",
			"cmd2", "cmd3",
			"arg1", "arg2",
		},
		VolumeMounts: []corev1.VolumeMount{volumeMount, toolsMount},
	}, {
		Image:   "step-3",
		Command: []string{entrypointBinary},
		Args: []string{
			"-wait_file", "/builder/tools/1",
			"-post_file", "/builder/tools/sidecar-kill",
			"-entrypoint", "cmd", "--",
			"arg1", "arg2",
		},
		VolumeMounts: []corev1.VolumeMount{toolsMount},
	}}
	got := orderContainers(sidecars, steps)
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff (-want, +got): %s", d)
	}
}
