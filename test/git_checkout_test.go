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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestGitPipelineRunFail is a test to ensure that the code extraction from github fails as expected when
// an invalid revision is passed on the pipelineresource.
func TestGitPipelineRunFail(t *testing.T) {
	gitSourceResourceName := "git-source-resource"
	gitTestTaskName := "git-check-task"
	gitTestPipelineName := "git-check-pipeline"
	gitTestPipelineRunName := "git-check-pipeline-run"

	t.Parallel()

	c, namespace := setup(t)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating Git PipelineResource %s", gitSourceResourceName)
	if _, err := c.PipelineResourceClient.Create(tb.PipelineResource(gitSourceResourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/tektoncd/pipeline"),
		tb.PipelineResourceSpecParam("Revision", "Idontexistrabbitmonkey"),
	))); err != nil {
		t.Fatalf("Failed to create Pipeline Resource `%s`: %s", gitSourceResourceName, err)
	}

	t.Logf("Creating Task %s", gitTestTaskName)
	if _, err := c.TaskClient.Create(tb.Task(gitTestTaskName, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("gitsource", v1alpha1.PipelineResourceTypeGit)),
		tb.Step("alpine/git", tb.StepScript("git --git-dir=/workspace/gitsource/.git show")),
	))); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", gitTestTaskName, err)
	}

	t.Logf("Creating Pipeline %s", gitTestPipelineName)
	if _, err := c.PipelineClient.Create(tb.Pipeline(gitTestPipelineName, tb.PipelineSpec(
		tb.PipelineDeclaredResource("git-repo", "git"),
		tb.PipelineTask("git-check", gitTestTaskName,
			tb.PipelineTaskInputResource("gitsource", "git-repo"),
		),
	))); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineName, err)
	}

	t.Logf("Creating PipelineRun %s", gitTestPipelineRunName)
	if _, err := c.PipelineRunClient.Create(tb.PipelineRun(gitTestPipelineRunName, tb.PipelineRunSpec(
		gitTestPipelineName,
		tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef(gitSourceResourceName)),
	))); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", gitTestPipelineRunName, err)
	}

	if err := WaitForPipelineRunState(c, gitTestPipelineRunName, timeout, PipelineRunSucceed(gitTestPipelineRunName), "PipelineRunCompleted"); err != nil {
		taskruns, err := c.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list for PipelineRun %s %s", gitTestPipelineRunName, err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				p, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(tr.Status.PodName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Error getting pod %q in namespace `%s`", tr.Status.PodName, namespace)
				}

				for _, stat := range p.Status.ContainerStatuses {
					if strings.HasPrefix(stat.Name, "step-git-source-"+gitSourceResourceName) {
						if stat.State.Terminated != nil {
							req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
							logContent, err := req.Do().Raw()
							if err != nil {
								t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
							}
							// Check for failure messages from fetch and pull in the log file
							if strings.Contains(strings.ToLower(string(logContent)), "couldn't find remote ref idontexistrabbitmonkeydonkey") &&
								strings.Contains(strings.ToLower(string(logContent)), "pathspec 'idontexistrabbitmonkeydonkey' did not match any file(s) known to git") {
								t.Logf("Found exepected errors when retrieving non-existent git revision")
							} else {
								t.Logf("Container `%s` log File: %s", stat.Name, logContent)
								t.Fatalf("The git code extraction did not fail as expected.  Expected errors not found in log file.")
							}
						}
					}
				}
			}
		}

	} else {
		t.Fatalf("PipelineRun succeeded when should have failed")
	}
}
