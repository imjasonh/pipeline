package pod

import (
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	mountPoint       = "/builder/tools"
	entrypointBinary = mountPoint + "/entrypoint"
)

var toolsMount = corev1.VolumeMount{
	Name:      "tools",
	MountPath: mountPoint,
}

// orderContainers returns a set of Containers representing the sidecars
// and step containers together, with entrypoint binary injected and with file
// signals wired up to enforce ordering.
//
// Containers must have Command specified; if the user didn't specify a
// command, we must have fetched the image's ENTRYPOINT before calling this
// method.
func orderContainers(sidecars, steps []corev1.Container) []corev1.Container {
	var out []corev1.Container
	var sidecarWaits []string
	sidecarKillFile := filepath.Join(mountPoint, "sidecar-kill")
	for i, s := range sidecars {
		// no wait files
		startFile := filepath.Join(mountPoint, fmt.Sprintf("sidecar-%d", i))
		sidecarWaits = append(sidecarWaits, startFile)
		argsForEntrypoint := []string{
			"-start_file", startFile,
			"-kill_file", sidecarKillFile,
		}

		cmd, args := s.Command, s.Args
		if len(cmd) == 0 {
			panic("oh no bad stuff") // TODO remove
		}
		if len(cmd) > 1 {
			args = append(cmd[1:], args...)
			cmd = []string{cmd[0]}
		}
		argsForEntrypoint = append(argsForEntrypoint, "-entrypoint", cmd[0], "--")
		argsForEntrypoint = append(argsForEntrypoint, args...)

		out = append(out, corev1.Container{
			Image:        s.Image,
			Command:      []string{entrypointBinary},
			Args:         argsForEntrypoint,
			VolumeMounts: append(s.VolumeMounts, toolsMount),
		})
	}

	for i, s := range steps {
		var argsForEntrypoint []string
		switch i {
		case 0:
			argsForEntrypoint = []string{
				// First step waits for all sidecar start files
				"-wait_file", strings.Join(sidecarWaits, ","),
				// Start next step.
				"-post_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i)),
			}
		case len(steps) - 1:
			argsForEntrypoint = []string{
				// Wait for previous step.
				"-wait_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i-1)),
				// Last step signals sidecars to exit.
				"-post_file", sidecarKillFile,
			}
		default:
			// All other steps wait for previous, write next.
			argsForEntrypoint = []string{
				"-wait_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i-1)),
				"-post_file", filepath.Join(mountPoint, fmt.Sprintf("%d", i)),
			}
		}

		cmd, args := s.Command, s.Args
		if len(cmd) == 0 {
			panic("oh no bad stuff") // TODO remove
		}
		if len(cmd) > 1 {
			args = append(cmd[1:], args...)
			cmd = []string{cmd[0]}
		}
		argsForEntrypoint = append(argsForEntrypoint, "-entrypoint", cmd[0], "--")
		argsForEntrypoint = append(argsForEntrypoint, args...)

		out = append(out, corev1.Container{
			Image:        s.Image,
			Command:      []string{entrypointBinary},
			Args:         argsForEntrypoint,
			VolumeMounts: append(s.VolumeMounts, toolsMount),
			// passthrough
			Env:        s.Env,
			WorkingDir: s.WorkingDir,
			Resources:  s.Resources,
			TTY:        s.TTY,
		})
	}
	return out
}
