// +build e2e

/*
Copyright 2020 The Tekton Authors

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

package test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestValidateRunTransition(t *testing.T) {
	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	run, err := c.RunClient.Create(&v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{Name: "run"},
		Spec: v1alpha1.RunSpec{
			Ref: &v1alpha1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
	})
	if err != nil {
		t.Fatalf("Creating run: %v", err)
	}

	t.Logf("Created Run %q", run.Name)

	// Cancel the Run.
	run.Spec.Status = "Cancelled"
	if run, err = c.RunClient.Update(run); err != nil {
		t.Fatalf("Update(cancelled): %v", err)
	}
	t.Log("Cancelled run")

	// Try to un-cancel the run.
	if run.Spec.Status != "Cancelled" {
		t.Fatalf("Run was not cancelled? .spec.status=%q", run.Spec.Status)
	}
	run.Spec.Status = ""
	if run, err = c.RunClient.Update(run); err == nil {
		t.Fatal("Update(un-cancelled): wanted error")
	} else {
		t.Logf("Got expected error un-cancelling Run: %v", err)
	}
}
