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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

func TestClusterResource(t *testing.T) {
	secretName := "hw-secret"
	configName := "hw-config"
	resourceName := "helloworld-cluster"
	taskName := "helloworld-cluster-task"
	taskRunName := "helloworld-cluster-taskrun"

	c, namespace := setup(t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	t.Logf("Creating secret %s", secretName)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		StringData: map[string]string{
			"cadatakey": "ca-cert",
			"tokenkey":  "token",
		},
	}
	if _, err := c.KubeClient.Kube.CoreV1().Secrets(namespace).Create(secret); err != nil {
		t.Fatalf("Failed to create Secret `%s`: %s", secretName, err)
	}

	t.Logf("Creating configMap %s", configName)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
		},
		Data: map[string]string{
			"test.data": `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: WTJFdFkyVnlkQW89
    server: https://1.1.1.1
  name: helloworld-cluster
contexts:
- context:
    cluster: helloworld-cluster
    user: test-user
  name: helloworld-cluster
current-context: helloworld-cluster
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: dG9rZW4K
`,
		},
	}
	if _, err := c.KubeClient.Kube.CoreV1().ConfigMaps(namespace).Create(configMap); err != nil {
		t.Fatalf("Failed to create configMap `%s`: %s", configName, err)
	}

	t.Logf("Creating cluster PipelineResource %q", resourceName)
	clusterResource := tb.PipelineResource(resourceName, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCluster,
		tb.PipelineResourceSpecParam("Name", "helloworld-cluster"),
		tb.PipelineResourceSpecParam("Url", "https://1.1.1.1"),
		tb.PipelineResourceSpecParam("username", "test-user"),
		tb.PipelineResourceSpecParam("password", "test-password"),
		tb.PipelineResourceSpecSecretParam("cadata", secretName, "cadatakey"),
		tb.PipelineResourceSpecSecretParam("token", secretName, "tokenkey"),
	))
	if _, err := c.PipelineResourceClient.Create(clusterResource); err != nil {
		t.Fatalf("Failed to create cluster Pipeline Resource `%s`: %s", resourceName, err)
	}

	t.Logf("Creating Task %s", taskName)
	task := tb.Task(taskName, tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("target-cluster", v1alpha1.PipelineResourceTypeCluster)),
		tb.TaskVolume("config-vol", tb.VolumeSource(corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configName,
				},
			},
		})),
		// Check files exist and are equal.
		tb.Step("ubuntu",
			tb.StepScript("cmp -b /workspace/helloworld-cluster/kubeconfig /config/test.data"),
			tb.StepVolumeMount("config-vol", "/config"),
		),
	))
	if _, err := c.TaskClient.Create(task); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	t.Logf("Creating TaskRun %s", taskRunName)
	taskRun := tb.TaskRun(taskRunName, tb.TaskRunSpec(
		tb.TaskRunTaskRef(taskName),
		tb.TaskRunInputs(tb.TaskRunInputsResource("target-cluster", tb.TaskResourceBindingRef(resourceName))),
	))
	if _, err := c.TaskRunClient.Create(taskRun); err != nil {
		t.Fatalf("Failed to create Taskrun `%s`: %s", taskRunName, err)
	}

	// Verify status of TaskRun (wait for it)
	if err := WaitForTaskRunState(c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunCompleted"); err != nil {
		t.Errorf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
}
