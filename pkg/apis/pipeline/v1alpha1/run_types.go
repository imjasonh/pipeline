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

package v1alpha1

import (
	"encoding/json"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var (
	runGroupVersionKind = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    pipeline.RunControllerName,
	}
)

// RunSpec defines the desired state of Run
type RunSpec struct {
	// +optional
	Ref *TaskRef `json:"ref,omitempty"`

	// +optional
	Params []v1beta1.Param `json:"params,omitempty"`

	// TODO:
	// - cancellation
	// - timeout
	// - inline task spec
	// - workspaces ?
}

// TODO: Move this to a Params type so other code can use it?
func (rs RunSpec) GetParam(name string) *v1beta1.Param {
	for _, p := range rs.Params {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

type RunStatus struct {
	duckv1beta1.Status `json:",inline"`

	// RunStatusFields inlines the status fields.
	RunStatusFields `json:",inline"`
}

// RunStatusFields holds the fields of Run's status.  This is defined
// separately and inlined so that other types can readily consume these fields
// via duck typing.
type RunStatusFields struct {
	// AdditionalFields holds arbitrary data reported by a controller.
	AdditionalFields json.RawMessage `json:"additionalFields,inline"`

	// StartTime is the time the build is actually started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the build completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Results reports any output result values to be consumed by later
	// tasks in a pipeline.
	// +optional
	Results []v1beta1.TaskRunResult `json:"results,omitempty"`
}

// Get returns the additional field value with the given key.
func (s *RunStatusFields) GetAdditionalField(k string) (interface{}, error) {
	var ad interface{}
	if len(s.AdditionalFields) == 0 {
		ad = map[string]interface{}{}
	} else {
		if err := json.Unmarshal(s.AdditionalFields, &ad); err != nil {
			return nil, err
		}
	}
	adm, ok := ad.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("AdditionalFields was not a map[string]interface{}, got %T", ad)
	}
	return adm[k], nil
}

// Set sets a new additional field with the given value.
func (s *RunStatusFields) SetAdditionalField(k string, d interface{}) error {
	var ad interface{}
	if len(s.AdditionalFields) == 0 {
		ad = map[string]interface{}{}
	} else {
		if err := json.Unmarshal(s.AdditionalFields, &ad); err != nil {
			return err
		}
	}
	adm, ok := ad.(map[string]interface{})
	if !ok {
		return fmt.Errorf("AdditionalFields was not a map[string]interface{}, got %T", ad)
	}
	adm[k] = d
	b, err := json.Marshal(adm)
	if err != nil {
		return err
	}
	s.AdditionalFields = b
	return nil
}

// TODO: clear key

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Run represents a single execution of a Custom Task.
//
// +k8s:openapi-gen=true
type Run struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec RunSpec `json:"spec,omitempty"`
	// +optional
	Status RunStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RunList contains a list of Run
type RunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Run `json:"items"`
}

// GetOwnerReference gets the task run as owner reference for any related objects
func (r *Run) GetOwnerReference() metav1.OwnerReference {
	return *metav1.NewControllerRef(r, runGroupVersionKind)
}

// HasPipelineRunOwnerReference returns true of Run has
// owner reference of type PipelineRun
func (r *Run) HasPipelineRunOwnerReference() bool {
	for _, ref := range r.GetOwnerReferences() {
		if ref.Kind == pipeline.PipelineRunControllerName {
			return true
		}
	}
	return false
}

// IsDone returns true if the Run's status indicates that it is done.
func (r *Run) IsDone() bool {
	return !r.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
}

// HasStarted function check whether taskrun has valid start time set in its status
func (r *Run) HasStarted() bool {
	return r.Status.StartTime != nil && !r.Status.StartTime.IsZero()
}

// IsSuccessful returns true if the Run's status indicates that it is done.
func (r *Run) IsSuccessful() bool {
	return r.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
}

// GetRunKey return the taskrun key for timeout handler map
func (r *Run) GetRunKey() string {
	// The address of the pointer is a threadsafe unique identifier for the taskrun
	return fmt.Sprintf("%s/%p", "Run", r)
}
