package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

const shellImage = "busybox"

func TestConvertScripts_NothingToConvert(t *testing.T) {
	got, _, _ := convertScripts(shellImage, []v1alpha1.Step{{Container: corev1.Container{
		Image: "step-1",
	}}, {Container: corev1.Container{
		Image: "step-2",
	}}})
	want := []corev1.Container{{
		Image: "step-1",
	}, {
		Image: "step-2",
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff (-want, +got): %s", d)
	}
}

func TestConvertScripts(t *testing.T) {
	names.TestingSeed()
	got, _, vms := convertScripts(shellImage, []v1alpha1.Step{{
		Script:    "script-1",
		Container: corev1.Container{Image: "step-1"},
	}, {
		// No script to convert here.
		Container: corev1.Container{Image: "step-2"},
	}, {
		Script: "script-3",
		Container: corev1.Container{
			Image:        "step-3",
			VolumeMounts: []corev1.VolumeMount{volumeMount}, // pre-existing volumeMount
		},
	}})
	want := []corev1.Container{{
		Name:    "place-scripts-mz4c7",
		Image:   shellImage,
		TTY:     true,
		Command: []string{"sh"},
		Args: []string{"-c", `tmpfile="/builder/scripts/script-0-mssqb"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-78c5n'
script-1
script-heredoc-randomly-generated-78c5n
tmpfile="/builder/scripts/script-2-6nl7g"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << 'script-heredoc-randomly-generated-j2tds'
script-3
script-heredoc-randomly-generated-j2tds
`},
		VolumeMounts: vms,
	}, {
		Image:        "step-1",
		Command:      []string{"/builder/scripts/script-0-mssqb"},
		VolumeMounts: vms,
	}, {
		Image: "step-2",
	}, {
		Image:        "step-3",
		Command:      []string{"/builder/scripts/script-2-6nl7g"},
		VolumeMounts: append([]corev1.VolumeMount{volumeMount}, vms...),
	}}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("Diff (-want, +got): %s", d)
	}
}
