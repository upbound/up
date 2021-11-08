// Copyright 2021 Upbound Inc
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

package dep

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
)

func TestResolveTag(t *testing.T) {

	type args struct {
		dep     metav1.Dependency
		fetcher fetcher
	}

	type want struct {
		tag string
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessTagFound": {
			reason: "Should return tag.",
			args: args{
				dep: metav1.Dependency{
					Provider: pointer.String("crossplane/provider-aws"),
					Version:  ">=v0.1.1",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"SuccessNoVersionSupplied": {
			reason: "Should return tag.",
			args: args{
				dep: metav1.Dependency{
					Provider: pointer.String("crossplane/provider-aws"),
					Version:  "",
				},
				fetcher: NewMockFetcher(
					WithTags(
						[]string{
							"v0.2.0",
							"alpha",
						},
					),
				),
			},
			want: want{
				tag: "v0.2.0",
			},
		},
		"ErrorInvalidTag": {
			reason: "Should return an error if dep has invalid constraint.",
			args: args{
				dep: metav1.Dependency{
					Provider: pointer.String("crossplane/provider-aws"),
					Version:  "alpha",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidConstraint),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New("improper constraint: alpha"), errInvalidConstraint),
			},
		},
		"ErrorInvalidReference": {
			reason: "Should return an error if dep has invalid provider.",
			args: args{
				dep: metav1.Dependency{
					Provider: pointer.String(""),
					Version:  "v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errInvalidProviderRef),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New("could not parse reference: "), errInvalidProviderRef),
			},
		},
		"ErrorFailedToFetchTags": {
			reason: "Should return an error if we could not fetch tags.",
			args: args{
				dep: metav1.Dependency{
					Provider: pointer.String("crossplane/provider-aws"),
					Version:  "v1.0.0",
				},
				fetcher: NewMockFetcher(
					WithError(
						errors.New(errFailedToFetchTags),
					),
				),
			},
			want: want{
				err: errors.Wrap(errors.New(errFailedToFetchTags), errFailedToFetchTags),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			r := NewResolver(WithFetcher(tc.args.fetcher))

			got, err := r.ResolveTag(context.Background(), tc.args.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.tag, got); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockFetcher struct {
	tags []string
	err  error
}

func NewMockFetcher(opts ...MockOption) *MockFetcher {
	f := &MockFetcher{}
	for _, o := range opts {
		o(f)
	}
	return f
}

// MockOption modifies the mock resolver.
type MockOption func(*MockFetcher)

func WithTags(tags []string) MockOption {
	return func(m *MockFetcher) {
		m.tags = tags
	}
}

func WithError(err error) MockOption {
	return func(m *MockFetcher) {
		m.err = err
	}
}

func (m *MockFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	return nil, nil
}
func (m *MockFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	return nil, nil
}
func (m *MockFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	if m.tags != nil {
		return m.tags, nil
	}
	return nil, m.err
}
