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
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestDAGPipelineRun creates a graph of arbitrary Tasks, then looks at the corresponding
// TaskRun start times to ensure they were run in the order intended, which is:
//                               |
//                        pipeline-task-1
//                       /               \
//   pipeline-task-2-parallel-1    pipeline-task-2-parallel-2
//                       \                /
//                        pipeline-task-3
//                               |
//                        pipeline-task-4
func TestDAGPipelineRun(t *testing.T) {
	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	// Create the Task that echoes text
	echoTask := tb.Task("echo-task", tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("repo", v1alpha1.PipelineResourceTypeGit),
			tb.InputsParamSpec("text", v1alpha1.ParamTypeString, tb.ParamSpecDescription("The text that should be echoed")),
		),
		tb.TaskOutputs(tb.OutputsResource("repo", v1alpha1.PipelineResourceTypeGit)),
		tb.Step("busybox", tb.StepScript("echo $(inputs.params.text)")),
		tb.Step("busybox", tb.StepScript("ln -s $(inputs.resources.repo.path) $(outputs.resources.repo.path)")),
	))
	if _, err := c.TaskClient.Create(echoTask); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	// Create the repo PipelineResource (doesn't really matter which repo we use)
	repoResource := tb.PipelineResource("repo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/githubtraining/example-basic"),
	))
	if _, err := c.PipelineResourceClient.Create(repoResource); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	// Intentionally declaring Tasks in a mixed up order to ensure the order
	// of execution isn't at all dependent on the order they are declared in
	pipeline := tb.Pipeline("dag-pipeline", tb.PipelineSpec(
		tb.PipelineDeclaredResource("repo", "git"),
		tb.PipelineTask("pipeline-task-3", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-2-parallel-1", "pipeline-task-2-parallel-2")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "wow"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-2", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")), tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "such parallel"),
		),
		tb.PipelineTask("pipeline-task-4", "echo-task",
			tb.RunAfter("pipeline-task-3"),
			tb.PipelineTaskInputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "very cloud native"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-1", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "much graph"),
		),
		tb.PipelineTask("pipeline-task-1", "echo-task",
			tb.PipelineTaskInputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("text", "how to ci/cd?"),
		),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create dag-pipeline: %s", err)
	}
	pipelineRun := tb.PipelineRun("dag-pipeline-run", tb.PipelineRunSpec("dag-pipeline",
		tb.PipelineRunResourceBinding("repo", tb.PipelineResourceBindingRef("repo")),
	))
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	t.Logf("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(c, "dag-pipeline-run", pipelineRunTimeout, PipelineRunSucceed("dag-pipeline-run"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	t.Logf("Verifying order of execution")
	taskRuns, err := c.TaskRunClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get TaskRuns (so that we could check when they executed): %v", err)
	}
	vefifyExpectedOrder(t, taskRuns.Items)
}

func vefifyExpectedOrder(t *testing.T, taskRuns []v1alpha1.TaskRun) {
	if len(taskRuns) != 5 {
		t.Fatalf("Expected 5 TaskRuns, got %d", len(taskRuns))
	}

	sort.Slice(taskRuns, func(i, j int) bool {
		return taskRuns[i].Status.StartTime.Time.Before(taskRuns[j].Status.StartTime.Time)
	})

	for i := 1; i <= 4; i++ {
		if !strings.HasPrefix(taskRuns[i].Name, fmt.Sprintf("dag-pipeline-run-pipeline-task-%d", i)) {
			t.Errorf("Expected task %d to be task %d, was %q", i, i, taskRuns[i].Name)
		}
	}

	// Check that the two tasks that can run in parallel did
	parallelDiff := taskRuns[2].Status.StartTime.Time.Sub(taskRuns[1].Status.StartTime.Time)
	if parallelDiff > (time.Second * 5) {
		t.Errorf("Expected parallel tasks to execute more or less at the same time, but they were %v apart", parallelDiff)
	}
}
