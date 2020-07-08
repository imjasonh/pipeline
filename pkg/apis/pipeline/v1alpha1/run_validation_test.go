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

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestRun_Invalid(t *testing.T) {
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
		want *apis.FieldError
	}{{
		name: "missing spec",
		run:  &v1alpha1.Run{},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "invalid metadata",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{Name: "run.name"},
		},
		want: &apis.FieldError{
			Message: "Invalid resource name: special character . must not be present",
			Paths:   []string{"metadata.name"},
		},
	}, {
		name: "missing ref",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: nil,
			},
		},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "missing apiVersion",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.apiVersion"),
	}, {
		name: "missing kind",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.kind"),
	}, {
		name: "non-unique params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: []v1beta1.Param{{
					Name:  "foo",
					Value: v1beta1.NewArrayOrString("foo"),
				}, {
					Name:  "foo",
					Value: v1beta1.NewArrayOrString("foo"),
				}},
			},
		},
		want: apis.ErrMultipleOneOf("spec.params"),
	}, {
		name: "invalid .spec.status",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Status: "BLAH",
			},
		},
		want: apis.ErrInvalidValue("BLAH", "spec.status"),
	}} {
		t.Run(c.name, func(t *testing.T) {
			err := c.run.Validate(context.Background())
			if d := cmp.Diff(err.Error(), c.want.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestRun_Valid(t *testing.T) {
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
	}{{
		name: "no params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
					Name:       "blah",
				},
			},
		},
	}, {
		name: "unnamed",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
			},
		},
	}, {
		name: "unique params",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: []v1beta1.Param{{
					Name:  "foo",
					Value: v1beta1.NewArrayOrString("foo"),
				}, {
					Name:  "bar",
					Value: v1beta1.NewArrayOrString("bar"),
				}},
			},
		},
	}, {
		name: "valid .spec.status Cancelled",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Status: "Cancelled",
			},
		},
	}, {
		name: "valid .spec.status TaskRunCancelled",
		run: &v1alpha1.Run{
			Spec: v1alpha1.RunSpec{
				Ref: &v1alpha1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Status: v1beta1.TaskRunSpecStatusCancelled,
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run.Validate(context.Background()); err != nil {
				t.Fatalf("validating valid Run: %v", err)
			}
		})
	}
}

func TestValidateRunTransition(t *testing.T) {
	for _, c := range []struct {
		name     string
		old, new v1alpha1.Run
		valid    bool
	}{{
		name:  "cannot un-cancel",
		old:   v1alpha1.Run{Spec: v1alpha1.RunSpec{Status: "Cancelled"}},
		new:   v1alpha1.Run{Spec: v1alpha1.RunSpec{Status: "Uncancelled"}},
		valid: false,
	}, {
		name: "cannot un-finish",
		old: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}},
		}}},
		new: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
			}},
		}}},
		valid: false,
	}, {
		name: "cannot succeed after failing",
		old: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		}}},
		new: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}},
		}}},
		valid: false,
	}, {
		name: "cannot fail after succeeding",
		old: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}},
		}}},
		new: v1alpha1.Run{Status: v1alpha1.RunStatus{Status: duckv1.Status{
			Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}},
		}}},
		valid: false,
		// TODO: cannot update reason/message for completed status
		// TODO: can update ongoing -> done
		// TODO: can add/update conditions of other types
	}} {
		t.Run(c.name, func(t *testing.T) {
			err := v1alpha1.ValidateRunTransition(c.old, c.new)
			gotValid := err == nil
			if gotValid != c.valid {
				t.Fatalf("ValidateRunTransition: got valid=%t, want valid=%t; %v", gotValid, c.valid, err)
			}
		})
	}
}
