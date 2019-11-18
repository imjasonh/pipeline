/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeStepTemplate(t *testing.T) {
	for _, c := range []struct {
		desc     string
		template *corev1.Container
		steps    []v1alpha1.Step
		want     []v1alpha1.Step
	}{{
		desc:     "nil-template",
		template: nil,
		steps: []v1alpha1.Step{{Container: corev1.Container{
			Image: "some-image",
		}}},
		want: []v1alpha1.Step{{Container: corev1.Container{
			Image: "some-image",
		}}},
	}, {
		desc: "not-overlapping",
		template: &corev1.Container{
			Command: []string{"/somecmd"},
		},
		steps: []v1alpha1.Step{{Container: corev1.Container{
			Image: "some-image",
		}}},
		want: []v1alpha1.Step{{Container: corev1.Container{
			Command: []string{"/somecmd"},
			Image:   "some-image",
		}}},
	}, {
		desc: "overwriting-one-field",
		template: &corev1.Container{
			Image:   "some-image",
			Command: []string{"/somecmd"},
		},
		steps: []v1alpha1.Step{{Container: corev1.Container{
			Image: "some-other-image",
		}}},
		want: []v1alpha1.Step{{Container: corev1.Container{
			Command: []string{"/somecmd"},
			Image:   "some-other-image",
		}}},
	}, {
		desc: "merge-and-overwrite-slice",
		template: &corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "ORIGINAL_VALUE",
			}},
		},
		steps: []v1alpha1.Step{{Container: corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "NEW_VALUE",
			}},
		}}},
		want: []v1alpha1.Step{{Container: corev1.Container{
			Env: []corev1.EnvVar{{
				Name:  "NEW_KEY",
				Value: "A_VALUE",
			}, {
				Name:  "KEEP_THIS",
				Value: "A_VALUE",
			}, {
				Name:  "SOME_KEY",
				Value: "NEW_VALUE",
			}},
		}}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := mergeStepTemplate(c.template, c.steps)
			if err != nil {
				t.Fatalf("MergeStepTemplate: %v", err)
			}

			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff (-want, +got): %s", d)
			}
		})
	}
}
