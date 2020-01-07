// +build e2e

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

package test

import (
	"testing"

	tb "github.com/tektoncd/pipeline/test/builder"
	knativetest "knative.dev/pkg/test"
)

// TestTaskRunPipelineRunStatus is an integration test that will
// verify a very simple "hello world" TaskRun and PipelineRun failure
// execution lead to the correct TaskRun status.
func TestTaskRunPipelineRunStatus(t *testing.T) {
	taskName := "task-name"
	taskRunName := "task-run-name"
	pipelineName := "pipeline-name"
	pipelineRunName := "pipeline-run-name"

	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := tb.Task(taskName, tb.TaskSpec(
		tb.Step("busybox", tb.StepScript("ls -la")),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := tb.TaskRun(taskRunName, tb.TaskRunSpec(
		tb.TaskRunTaskRef(taskName), tb.TaskRunServiceAccountName("inexistent"),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunFailed(taskRunName), "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	pipeline := tb.Pipeline(pipelineName,
		tb.PipelineSpec(tb.PipelineTask("foo", taskName)),
	)
	pipelineRun := tb.PipelineRun(pipelineRunName, tb.PipelineRunSpec(
		pipelineName, tb.PipelineRunServiceAccountName("inexistent"),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRunName, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(c, pipelineRunName, pipelineRunTimeout, PipelineRunFailed(pipelineRunName), "BuildValidationFailed"); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}
}
