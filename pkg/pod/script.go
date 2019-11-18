package pod

import (
	"fmt"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const scriptsDir = "/builder/scripts"

// convertScripts converts any steps that specify a Script field into a normal Container.
//
// It does this by prepending a container that writes specified Script bodies
// to executable files in a shared volumeMount, then produces Containers that
// simply run those executable files.
func convertScripts(shellImage string, steps []v1alpha1.Step) ([]corev1.Container, []corev1.Volume, []corev1.VolumeMount) {
	// Generate volumeMount names, to avoid collisions.
	scriptsVolumeName := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("scripts")
	scriptsVolumes := []corev1.Volume{{
		Name:         scriptsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}}
	scriptsVolumeMounts := []corev1.VolumeMount{{
		Name:      scriptsVolumeName,
		MountPath: scriptsDir,
	}}

	placeScripts := false
	placeScriptsStep := corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("place-scripts"),
		Image:        shellImage,
		TTY:          true,
		Command:      []string{"sh"},
		Args:         []string{"-c", ""},
		VolumeMounts: scriptsVolumeMounts,
	}

	var out []corev1.Container
	for i, s := range steps {
		if s.Script == "" {
			// Nothing to convert.
			out = append(out, s.Container)
			continue
		}

		if placeScripts == false {
			// If this is the first step that uses a script,
			// prepend the place-scripts step to the output.
			placeScripts = true
			out = append([]corev1.Container{placeScriptsStep}, out...)
		}

		// Append to the place-scripts script to place the
		// script file in a known location in the scripts volume.
		tmpFile := filepath.Join(scriptsDir, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("script-%d", i)))
		// heredoc is the "here document" placeholder string
		// used to cat script contents into the file. Typically
		// this is the string "EOF" but if this value were
		// "EOF" it would prevent users from including the
		// string "EOF" in their own scripts. Instead we
		// randomly generate a string to (hopefully) prevent
		// collisions.
		heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("script-heredoc-randomly-generated")
		placeScriptsStep.Args[1] += fmt.Sprintf(`tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`, tmpFile, heredoc, s.Script, heredoc)

		out = append(out, corev1.Container{
			Image:        s.Image,
			Command:      []string{tmpFile},
			Args:         nil, // no args allowed with scripts.
			VolumeMounts: append(s.VolumeMounts, scriptsVolumeMounts...),
			// passthrough
			Env:        s.Env,
			WorkingDir: s.WorkingDir,
			Resources:  s.Resources,
			TTY:        s.TTY,
		})
	}

	return out, scriptsVolumes, scriptsVolumeMounts
}
