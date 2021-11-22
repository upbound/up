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

package manager

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
)

func TestResolveTransitiveDependencies(t *testing.T) {
	// SUT - recursively reading in meta and pulling deps using the manager

	fs := afero.NewMemMapFs()
	c, _ := cache.NewLocal(cache.WithFS(fs), cache.WithRoot("/tmp/cache"))

	type depMeta struct {
		dep  v1beta1.Dependency
		meta runtime.Object
	}

	type args struct {
		// root represents the root dependency and its corresponding meta file
		// that may or may not have transitive dependencies
		root depMeta
		// leaf represents the leaf dependency and its corresponding meta file
		leaf depMeta
	}

	type want struct {
		// entries we expect to exist in system given the above args
		entries []v1beta1.Dependency
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoTransitiveDependencies": {
			reason: "Should successfully store the root dependency.",
			args: args{
				root: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{},
						},
					},
				},
			},
			want: want{
				entries: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
				},
			},
		},
		"TransitiveDependency": {
			reason: "Should successfully store both the root and the transitive dependency.",
			args: args{
				root: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{
								DependsOn: []metav1.Dependency{
									{
										Provider: pointer.String("crossplane/provider-aws-dependency"),
										Version:  "v1.10.0",
									},
								},
							},
						},
					},
				},
				leaf: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws-dependency",
						Constraints: "v1.10.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{},
						},
					},
				},
			},
			want: want{
				entries: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					{
						Package:     "crossplane/provider-aws-dependency",
						Constraints: "v1.10.0",
					},
				},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			ref, _ := name.ParseReference(dep.ImgTag(tc.args.root.dep))
			lref, _ := name.ParseReference(dep.ImgTag(tc.args.leaf.dep))

			sut := New(
				WithCache(c),
				WithResolver(
					dep.NewResolver(
						func(r *dep.Resolver) {
							r.F = NewMockFetcher(
								WithMeta(ref, tc.args.root.meta),
								WithMeta(lref, tc.args.leaf.meta),
							)
						},
					),
				),
			)

			_, err := sut.Resolve(context.Background(), tc.args.root.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nResolveTransitiveDependencies(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			for _, e := range tc.want.entries {
				// for each expressed entry, we should not get a NotExists
				_, err := sut.c.Get(e)

				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nResolveTransitiveDependencies(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

type MockFetcher struct {
	pkgMeta map[name.Reference]runtime.Object
	tags    []string
	err     error
}

func NewMockFetcher(opts ...MockFetcherOption) *MockFetcher {
	f := &MockFetcher{
		pkgMeta: make(map[name.Reference]runtime.Object),
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// MockFetcherOption modifies the mock resolver.
type MockFetcherOption func(*MockFetcher)

func WithMeta(ref name.Reference, meta runtime.Object) MockFetcherOption {
	return func(m *MockFetcher) {
		m.pkgMeta[ref] = meta
	}
}

func (m *MockFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	meta, ok := m.pkgMeta[ref]
	if !ok {
		return nil, errors.New("entry does not exist in pkgMeta map")
	}
	return newPackageImage(meta), nil
}
func (m *MockFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	h, _ := v1.NewHash("test")

	return &v1.Descriptor{
		Digest: h,
	}, nil
}
func (m *MockFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	if m.tags != nil {
		return m.tags, nil
	}
	return nil, m.err
}

func newPackageImage(meta runtime.Object) v1.Image {
	pack, _ := yaml.Marshal(meta)

	fmt.Println(string(pack))
	r := bytes.NewReader(pack)

	buf := new(bytes.Buffer)

	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: int64(len(pack)),
	}

	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, r)
	_ = tw.Close()
	packLayer, _ := tarball.LayerFromReader(buf)
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	return packImg
}
