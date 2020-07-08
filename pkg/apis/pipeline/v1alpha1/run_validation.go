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

package v1alpha1

import (
	"context"
	"fmt"

	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Run)(nil)

// Validate taskrun
func (r *Run) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(r.GetObjectMeta()).ViaField("metadata"); err != nil {
		return err
	}
	return r.Spec.Validate(ctx)
}

// Validate Run spec
func (rs *RunSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(rs, &RunSpec{}) {
		return apis.ErrMissingField("spec")
	}

	if rs.Ref == nil {
		return apis.ErrMissingField("spec.ref")
	}
	if rs.Ref.APIVersion == "" {
		return apis.ErrMissingField("spec.ref.apiVersion")
	}
	if rs.Ref.Kind == "" {
		return apis.ErrMissingField("spec.ref.kind")
	}

	switch rs.Status {
	case "", "Cancelled", v1beta1.TaskRunSpecStatusCancelled:
		// valid values.
	default:
		return apis.ErrInvalidValue(rs.Status, "spec.status")
	}

	if err := validateParameters("spec.params", rs.Params); err != nil {
		return err
	}

	return nil
}

// ValidateRunTransition returns an error if the transition from old to new is
// invalid.
func ValidateRunTransition(old, new Run) error {
	// Don't allow .spec.status to change from a value to any other value.
	if old.Spec.Status != "" && old.Spec.Status != new.Spec.Status {
		return fmt.Errorf("update .spec.status from %q to %q", old.Spec.Status, new.Spec.Status)
	}

	// Don't allow finished status going from finished -> ongoing.
	if old.IsDone() && !new.IsDone() {
		return fmt.Errorf("cannot update .status.conditions from finished to ongoing")
	}
	// Don't allow finished status going from success -> failure, or vice versa.
	if old.IsDone() &&
		old.Status.GetCondition(apis.ConditionSucceeded) != new.Status.GetCondition(apis.ConditionSucceeded) {
		return fmt.Errorf("cannot update .status.conditions to change successful status")
	}

	return nil
}
