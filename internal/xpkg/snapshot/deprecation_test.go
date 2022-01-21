// Copyright 2022 Upbound Inc
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

package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

func TestAPIVersionDeprecation(t *testing.T) {
	type args struct {
		o runtime.Object
	}
	type want struct {
		err *validator.Validation
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"V1alpha1Configuration": {
			reason: "Should return a deprecation warning.",
			args: args{
				o: &metav1alpha1.Configuration{
					TypeMeta: v1.TypeMeta{
						Kind:       metav1alpha1.ConfigurationKind,
						APIVersion: metav1alpha1.SchemeGroupVersion.String(),
					},
				},
			},
			want: want{
				err: &validator.Validation{
					TypeCode: validator.WarningTypeCode,
					Message:  "meta.pkg.crossplane.io/v1alpha1 is deprecated in favor of meta.pkg.crossplane.io/v1",
					Name:     "apiVersion",
				},
			},
		},
		"V1alpha1Provider": {
			reason: "Should return a deprecation warning.",
			args: args{
				o: &metav1alpha1.Provider{
					TypeMeta: v1.TypeMeta{
						Kind:       metav1alpha1.ProviderKind,
						APIVersion: metav1alpha1.SchemeGroupVersion.String(),
					},
				},
			},
			want: want{
				err: &validator.Validation{
					TypeCode: validator.WarningTypeCode,
					Message:  "meta.pkg.crossplane.io/v1alpha1 is deprecated in favor of meta.pkg.crossplane.io/v1",
					Name:     "apiVersion",
				},
			},
		},
		"V1Configuration": {
			reason: "Should return a deprecation warning.",
			args: args{
				o: &metav1.Configuration{
					TypeMeta: v1.TypeMeta{
						Kind:       metav1.ConfigurationKind,
						APIVersion: metav1.SchemeGroupVersion.String(),
					},
				},
			},
		},
		"V1Provider": {
			reason: "Should return a deprecation warning.",
			args: args{
				o: &metav1.Provider{
					TypeMeta: v1.TypeMeta{
						Kind:       metav1.ProviderKind,
						APIVersion: metav1.SchemeGroupVersion.String(),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			err := validateAPIVersion(tc.args.o)

			if err != nil {
				if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nAPIVersionDeprecation(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
