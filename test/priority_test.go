package test

import (
	"fmt"
	"log"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	schedv1beta1 "k8s.io/api/scheduling/v1beta1"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

func TestPriorityClass(t *testing.T) {
	t.Parallel()
	c, namespace := setup(t)
	log.Printf("Namespace: %s", namespace)
	knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
	defer tearDown(t, c, namespace)

	// Resource-limit the namespace to 1 pod.
	if _, err := c.KubeClient.Kube.CoreV1().ResourceQuotas(namespace).Create(&corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: "resource-limit",
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{"pods": resource.MustParse("1")},
		},
	}); err != nil {
		t.Fatalf("Creating resource quota: %v", err)
	}

	// Create high and low priority classes.
	for _, lim := range []struct {
		name     string
		priority int32
	}{{
		"low-priority", 1000000,
	}, {
		"high-priority", 2000000,
	}} {
		if _, err := c.PriorityClassClient.Create(&schedv1beta1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: lim.name,
			},
			Value: lim.priority,
		}); err != nil && !kuberrors.IsAlreadyExists(err) {
			t.Fatalf("Creating priority class %q: %v", lim.name, err)
		}
	}

	isRunning := func(tr *v1alpha1.TaskRun) (bool, error) {
		if c := tr.Status.GetCondition(apis.ConditionSucceeded); c != nil {
			if c.IsTrue() || c.IsFalse() {
				log.Println("taskrun is done")
				return true, fmt.Errorf("taskRun %q already finished", tr.Name)
			} else if c.IsUnknown() && (c.Reason == "Running" || c.Reason == "Pending") {
				return true, nil
			}
		}
		log.Println("taskrun is not running")
		return false, nil
	}

	// Create a low-priority TaskRun and wait for it to start.
	lowPriority := "low-priority"
	if _, err := c.TaskRunClient.Create(&v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: lowPriority},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{
					Container: corev1.Container{
						Image: "ubuntu",
					},
					Script: "sleep infinity",
				}},
			},
			PodTemplate: v1alpha1.PodTemplate{PriorityClassName: &lowPriority},
		},
	}); err != nil {
		t.Fatalf("Creating TaskRun %q: %v", lowPriority, err)
	}
	log.Println("Created low-priority")
	if err := WaitForTaskRunState(c, lowPriority, isRunning, "TaskRunRunning"); err != nil {
		t.Fatalf("Waiting for low-priority TaskRun to start: %v", err)
	}
	log.Println("low-priority is running")

	// Create a high-priority TaskRun and wait for it to start.
	highPriority := "high-priority"
	if _, err := c.TaskRunClient.Create(&v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: highPriority},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []v1alpha1.Step{{
					Container: corev1.Container{
						Image: "ubuntu",
					},
					Script: "sleep infinity",
				}},
			},
			PodTemplate: v1alpha1.PodTemplate{PriorityClassName: &highPriority},
		},
	}); err != nil {
		t.Fatalf("Creating TaskRun %q: %v", highPriority, err)
	}
	log.Println("Created high-priority")
	if err := WaitForTaskRunState(c, highPriority, isRunning, "TaskRunRunning"); err != nil {
		t.Fatalf("Waiting for high-priority TaskRun to start: %v", err)
	}
	log.Println("high-priority is running")

	// See that the low-priority TaskRun is Failed.
	tr, err := c.TaskRunClient.Get(lowPriority, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting low-priority TaskRun: %v", err)
	}
	if c := tr.Status.GetCondition(apis.ConditionSucceeded); c.Reason != "Failed" {
		t.Fatalf("Low-priority TaskRun was not Failed: %+v", c)
	}
}
