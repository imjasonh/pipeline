package pod

import (
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	homeDir      = "/builder/home"
	workspaceDir = "/workspace"
)

var implicitEnvVars = []corev1.EnvVar{{
	Name:  "HOME",
	Value: homeDir,
}}

// setWorkspaceAndHome sets the default workingDir to /workspace, and $HOME to
// /builder/home, both of which are backed by a Volume.
//
// The names of these Volumes are generated and returned by this method, to
// prevent collisions.
//
// If a step specifies a value for HOME, it will be specified last, which
// takes precendence.
// If a step specifies a VolumeMount at either /workspace or /builder/home,
// that VolumeMount will take precedence.
func setWorkspaceAndHome(steps []v1alpha1.Step) ([]v1alpha1.Step, []corev1.Volume, []corev1.VolumeMount) {
	// Generate implicit volume mount names, to avoid collisions.
	workspaceVolume := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("workspace")
	homeVolume := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("home")
	implicitVolumes := []corev1.Volume{{
		Name:         workspaceVolume,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}, {
		Name:         homeVolume,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}
	implicitVolumeMounts := []corev1.VolumeMount{{
		Name:      workspaceVolume,
		MountPath: workspaceDir,
	}, {
		Name:      homeVolume,
		MountPath: homeDir,
	}}

	var out []v1alpha1.Step
	for _, s := range steps {
		env := append(implicitEnvVars, s.Env...)

		// Add implicit volume mounts, unless the user has requested
		// their own volume mount at that path.
		volumeMounts := s.VolumeMounts[:] // copy
		requestedVolumeMounts := map[string]bool{}
		for _, vm := range volumeMounts {
			requestedVolumeMounts[filepath.Clean(vm.MountPath)] = true
		}
		for _, imp := range implicitVolumeMounts {
			if !requestedVolumeMounts[filepath.Clean(imp.MountPath)] {
				volumeMounts = append(volumeMounts, imp)
			}
		}

		workingDir := s.WorkingDir
		if workingDir == "" {
			workingDir = workspaceDir
		}

		out = append(out, v1alpha1.Step{
			Container: corev1.Container{
				Env:          env,
				VolumeMounts: volumeMounts,
				WorkingDir:   workingDir,
				// passthrough
				Image:   s.Image,
				Command: s.Command,
				Args:    s.Args,
			},
			Script: s.Script,
		})
	}
	return out, implicitVolumes, implicitVolumeMounts
}
