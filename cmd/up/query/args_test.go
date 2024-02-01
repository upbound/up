// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package query

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/errors"
)

func TestParseTypesAndNames(t *testing.T) {
	tests := map[string]struct {
		reason   string
		args     []string
		want     []typeGroupNames
		wantErrs bool
	}{
		"none": {
			reason: "No arguments",
		},
		"just type": {
			reason: "gives tuple without names",
			args:   []string{"pods"},
			want:   []typeGroupNames{{Type: "pods"}},
		},
		"type and name": {
			reason: "gives tuple with name",
			args:   []string{"pods", "foo"},
			want:   []typeGroupNames{{Type: "pods", Names: []string{"foo"}}},
		},
		"multiple types": {
			reason: "gives multiple tuples without names",
			args:   []string{"pods,svc"},
			want:   []typeGroupNames{{Type: "pods"}, {Type: "svc"}},
		},
		"multiple types and names": {
			reason: "gives multiple tuples with names",
			args:   []string{"pods,svc", "foo", "bar"},
			want:   []typeGroupNames{{Type: "pods", Names: []string{"foo", "bar"}}, {Type: "svc", Names: []string{"foo", "bar"}}},
		},
		"multiple types and names with space": {
			reason: "gives multiple tuples with names",
			args:   []string{"pods,svc", "foo", "bar"},
			want:   []typeGroupNames{{Type: "pods", Names: []string{"foo", "bar"}}, {Type: "svc", Names: []string{"foo", "bar"}}},
		},
		"multiple names with comma": {
			reason:   "gives error",
			args:     []string{"pods", "foo,bar"},
			wantErrs: true,
		},
		"multiple types and names with comma and space": {
			reason:   "gives error",
			args:     []string{"pods,svc", "foo,bar"},
			wantErrs: true,
		},
		"type/name": {
			reason: "gives tuple with name",
			args:   []string{"pods/foo"},
			want:   []typeGroupNames{{Type: "pods", Names: []string{"foo"}}},
		},
		"multiple type/names with comma": {
			reason:   "gives multiple tuples with names",
			args:     []string{"pods/foo,svc/bar"},
			wantErrs: true,
		},
		"multiple type/names with space": {
			reason: "gives multiple tuples with names",
			args:   []string{"pods/foo", "svc/bar"},
			want:   []typeGroupNames{{Type: "pods", Names: []string{"foo"}}, {Type: "svc", Names: []string{"bar"}}},
		},
		"type/name1,name2": {
			reason:   "gives multiple tuples with names",
			args:     []string{"pods/foo,bar"},
			wantErrs: true,
		},
		"mixed types and type/names": {
			reason:   "gives errors",
			args:     []string{"pods,svc/foo"},
			wantErrs: true,
		},
		"type.group": {
			reason: "appended group",
			args:   []string{"customresourcedefinitions.apiextensions.k8s.io"},
			want:   []typeGroupNames{{Type: "customresourcedefinitions", Group: "apiextensions.k8s.io"}},
		},
		"type.group with name": {
			reason: "appended group",
			args:   []string{"customresourcedefinitions.apiextensions.k8s.io", "foo"},
			want:   []typeGroupNames{{Type: "customresourcedefinitions", Group: "apiextensions.k8s.io", Names: []string{"foo"}}},
		},
		"type.group/name": {
			reason: "appended group",
			args:   []string{"customresourcedefinitions.apiextensions.k8s.io/foo"},
			want:   []typeGroupNames{{Type: "customresourcedefinitions", Group: "apiextensions.k8s.io", Names: []string{"foo"}}},
		},
		"mixed case": {
			reason: "case is passed through",
			args:   []string{"Pods.GrouP/Foo"},
			want:   []typeGroupNames{{Type: "Pods", Group: "GrouP", Names: []string{"Foo"}}},
		},
		"type.v1.group": {
			reason:   "works in kubectl get, but not here because we don't support versioning",
			args:     []string{"deployments.v1.apps"},
			wantErrs: true,
		},
		"type.v1": {
			reason:   "needs a group, does not work with kubectl get either",
			args:     []string{"deployments.v1"},
			wantErrs: true,
		},
		"type.vuvuzela.group": {
			reason: "v-word that is not a version",
			args:   []string{"deployments.vuvuzela.apps"},
			want:   []typeGroupNames{{Type: "deployments", Group: "vuvuzela.apps"}},
		},
		"type.V1.group": {
			reason: "v-word that is not a version",
			args:   []string{"deployments.V1.apps"},
			want:   []typeGroupNames{{Type: "deployments", Group: "V1.apps"}},
		},
		"type.v.group": {
			reason: "v is no version",
			args:   []string{"deployments.v.apps"},
			want:   []typeGroupNames{{Type: "deployments", Group: "v.apps"}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, errs := ParseTypesAndNames(tt.args...)

			if diff := cmp.Diff(tt.wantErrs, len(errs) > 0); diff != "" {
				t.Errorf("%s\n\nResourceTypeOrNameArgs(%s): -want err, +got err:\n%s\nerrs: %s", tt.reason, strings.Join(tt.args, ", "), diff, errors.NewAggregate(errs))
			}
			if len(errs) == 0 {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("%s\n\nResourceTypeOrNameArgs(%s): -want, +got:\n%s", tt.reason, strings.Join(tt.args, ", "), diff)
				}
			}
		})
	}
}
