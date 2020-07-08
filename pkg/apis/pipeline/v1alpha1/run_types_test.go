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
	json "encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestGetParams(t *testing.T) {
	for _, c := range []struct {
		desc string
		spec v1alpha1.RunSpec
		name string
		want *v1beta1.Param
	}{{
		desc: "no params",
		spec: v1alpha1.RunSpec{},
		name: "anything",
		want: nil,
	}, {
		desc: "found",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: v1beta1.NewArrayOrString("bar"),
		},
	}, {
		desc: "not found",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}},
		},
		name: "bar",
		want: nil,
	}, {
		// This shouldn't happen since it's invalid, but just in
		// case, GetParams just returns the first param it finds with
		// the specified name.
		desc: "multiple with same name",
		spec: v1alpha1.RunSpec{
			Params: []v1beta1.Param{{
				Name:  "first",
				Value: v1beta1.NewArrayOrString("blah"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("bar"),
			}, {
				Name:  "foo",
				Value: v1beta1.NewArrayOrString("second bar"),
			}},
		},
		name: "foo",
		want: &v1beta1.Param{
			Name:  "foo",
			Value: v1beta1.NewArrayOrString("bar"),
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := c.spec.GetParam(c.name)
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("Diff(-want,+got): %v", d)
			}
		})
	}
}

func TestAdditionalFields(t *testing.T) {
	r := &v1alpha1.Run{}

	// Empty additionalFields -> nil result, no error.
	got, err := r.Status.GetAdditionalField("not-found")
	if err != nil {
		t.Fatalf("GetAdditionalField('not-found'): %v", err)
	}
	if got != nil {
		t.Fatalf("GetAdditionalField('not-found') got %v, want nil", got)
	}

	// Set a numerical field value.
	if err := r.Status.SetAdditionalField("foo", 123); err != nil {
		t.Fatalf("SetAdditionalField('foo'): %v", err)
	}
	// Get the field value.
	got, err = r.Status.GetAdditionalField("foo")
	if err != nil {
		t.Fatalf("GetAdditionalField('foo'): %v", err)
	}
	if got == nil {
		t.Fatal("GetAdditionalField('foo') was nil")
	}
	// Value is float64 even though it was provided as int...
	if gotv, ok := got.(float64); !ok {
		t.Fatalf("GetAdditionalField('foo') was not float64: %T", got)
	} else if gotv != 123 {
		t.Fatalf("GetAdditionalField('foo') got %v, want 123", gotv)
	}

	// Set struct field value.
	type pair struct{ A, B string }
	v := pair{"a", "b"}
	if err := r.Status.SetAdditionalField("bar", v); err != nil {
		t.Fatalf("SetAdditionalField('bar'): %v", err)
	}
	// Get the field value.
	got, err = r.Status.GetAdditionalField("bar")
	if err != nil {
		t.Fatalf("GetAdditionalField('bar'): %v", err)
	}
	if got == nil {
		t.Fatal("GetAdditionalField('bar') was nil")
	}
	// Value is map[string]interface{} even though it was provided as struct...
	if gotv, ok := got.(map[string]interface{}); !ok {
		t.Fatalf("GetAdditionalField('bar') was not map[string]interface{}: %T", got)
	} else {
		want := map[string]interface{}{"A": "a", "B": "b"}
		if d := cmp.Diff(want, gotv); d != "" {
			t.Fatalf("GetAdditionalField('bar'): Diff(-want,+got): %s", d)
		}
	}

	// Check the exact JSON bytes stored.
	want := `{"bar":{"A":"a","B":"b"},"foo":123}`
	if d := cmp.Diff(want, string(r.Status.AdditionalFields)); d != "" {
		t.Fatalf("AdditionalFields: Diff(-want,+got): %s", d)
	}

	// Clear an unknown field, no error.
	if err := r.Status.ClearAdditionalField("not-found"); err != nil {
		t.Fatalf("ClearAdditionalField('not-found'): %v", err)
	}

	// Clear a set field.
	if err := r.Status.ClearAdditionalField("foo"); err != nil {
		t.Fatalf("ClearAdditionalField('foo'): %v", err)
	}
	got, err = r.Status.GetAdditionalField("foo")
	if err != nil {
		t.Fatalf("GetAdditionalField('foo'): %v", err)
	}
	if got != nil {
		t.Fatalf("GetAdditionalField('foo') got %v, want nil", got)
	}
}

func TestAdditionalFields_DirectAccess(t *testing.T) {
	r := &v1alpha1.Run{
		Status: v1alpha1.RunStatus{
			RunStatusFields: v1alpha1.RunStatusFields{
				AdditionalFields: json.RawMessage(`{"foo":"bar"}`),
			},
		},
	}
	got, err := r.Status.GetAdditionalField("foo")
	if err != nil {
		t.Fatalf("GetAdditionalField('foo'): %v", err)
	}
	if gotv, ok := got.(string); !ok {
		t.Fatalf("GetAdditionalField('foo') was not string: %T", got)
	} else if gotv != "bar" {
		t.Fatalf("GetAdditionalField('foo') got %q, want %q", gotv, "bar")
	}

	// Directly set bytes to invalid JSON -> error getting value.
	r.Status.AdditionalFields = json.RawMessage(`{[[[}`)
	if _, err := r.Status.GetAdditionalField("foo"); err == nil {
		t.Fatal("GetAdditionalField wanted error, got nil")
	} else {
		t.Logf("Invalid JSON error: %v", err)
	}

	// Directly set bytes to valid JSON that isn't a map[string]interface{} -> error getting value.
	r.Status.AdditionalFields = json.RawMessage(`["a","b","c"]`)
	if _, err := r.Status.GetAdditionalField("foo"); err == nil {
		t.Fatal("GetAdditionalField wanted error, got nil")
	} else {
		t.Logf("Valid non-map JSON error: %v", err)
	}
}
