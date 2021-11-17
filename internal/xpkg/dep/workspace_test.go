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
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/utils/pointer"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestUpsert(t *testing.T) {
	type args struct {
		fs       afero.Fs
		metaFile runtime.Object
		dep      v1beta1.Dependency
	}

	type want struct {
		deps []metav1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddEntryNoPrior": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				fs: afero.NewMemMapFs(),
				dep: NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ConfigurationPackageType),
				),
				metaFile: &metav1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-gcp"),
						Version:       "v1.0.0",
					},
				},
			},
		},
		"InsertNewEntry": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				fs: afero.NewMemMapFs(),
				dep: NewWithType(
					"crossplane/provider-gcp@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  ">=1.0.5",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  ">=1.0.5",
					},
					{
						Provider: pointer.String("crossplane/provider-gcp"),
						Version:  "v1.0.0",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// init workspace that is specific to this test
			ws := newTestWS(tc.args.fs)
			// write the meta file to the fs
			_ = ws.writeMetaPkg(tc.args.metaFile)

			err := ws.Upsert(tc.args.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsert(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			p, _ := ws.readPkgMeta()
			pkg := p.(metav1.Pkg)

			resultDeps := pkg.GetDependencies()

			if diff := cmp.Diff(tc.want.deps, resultDeps, cmpopts.SortSlices(func(i, j int) bool {
				return *resultDeps[i].Provider < *resultDeps[j].Provider
			})); diff != "" {
				t.Errorf("\n%s\nUpsert(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestUpsertDeps(t *testing.T) {
	type args struct {
		dep v1beta1.Dependency
		pkg runtime.Object
	}

	type want struct {
		deps []metav1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyDependencyList": {
			reason: "Should return an updated deps list with the included provider.",
			args: args{
				dep: NewWithType(
					"crossplane/provider-aws@v1.0.0",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Configuration{
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.StringPtr("crossplane/provider-aws"),
						Version:  "v1.0.0",
					},
				},
			},
		},
		"InsertIntoDependencyList": {
			reason: "Should return an updated deps list with 2 entries.",
			args: args{
				dep: NewWithType(
					"crossplane/provider-gcp@v1.0.1",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Configuration{
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.0",
					},
					{
						Provider: pointer.String("crossplane/provider-gcp"),
						Version:  "v1.0.1",
					},
				},
			},
		},
		"UpdateDependencyList": {
			reason: "Should return an updated deps list with the provider version updated.",
			args: args{
				dep: NewWithType(
					"crossplane/provider-aws@v1.0.1",
					string(v1beta1.ConfigurationPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-aws"),
									Version:       "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.1",
					},
				},
			},
		},
		"UseDefaultTag": {
			reason: "Should return an error indicating the package name is invalid.",
			args: args{
				dep: NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  defaultVer,
					},
				},
			},
		},
		"DuplicateDep": {
			reason: "Should return an error indicating duplicate dependencies detected.",
			args: args{
				dep: NewWithType(
					"crossplane/provider-aws",
					string(v1beta1.ProviderPackageType),
				),
				pkg: &metav1.Provider{
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.1",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.New(errMetaContainsDupeDep),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			err := upsertDeps(tc.args.dep, tc.args.pkg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if tc.want.deps != nil {
				p := tc.args.pkg.(metav1.Pkg)
				if diff := cmp.Diff(tc.want.deps, p.GetDependencies()); diff != "" {
					t.Errorf("\n%s\nUpsertDeps(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestRWMetaFile(t *testing.T) {

	cfgMetaFile := &metav1.Configuration{
		TypeMeta: apimetav1.TypeMeta{
			APIVersion: "meta.pkg.crossplane.io/v1",
			Kind:       "Configuration",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "getting-started-with-aws",
		},
		Spec: metav1.ConfigurationSpec{
			MetaSpec: metav1.MetaSpec{
				Crossplane: &metav1.CrossplaneConstraints{
					Version: ">=1.0.0-0",
				},
				DependsOn: []metav1.Dependency{
					{
						Configuration: pointer.String("crossplane/provider-aws"),
						Version:       "v1.0.0",
					},
				},
			},
		},
	}

	providerMetaFile := &metav1.Provider{
		TypeMeta: apimetav1.TypeMeta{
			APIVersion: "meta.pkg.crossplane.io/v1",
			Kind:       "Provider",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "getting-started-with-aws",
		},
		Spec: metav1.ProviderSpec{
			Controller: metav1.ControllerSpec{
				Image: "crossplane/provider-aws",
			},
			MetaSpec: metav1.MetaSpec{
				Crossplane: &metav1.CrossplaneConstraints{
					Version: ">=1.0.0-0",
				},
				DependsOn: []metav1.Dependency{
					{
						Provider: pointer.String("crossplane/provider-aws"),
						Version:  "v1.0.0",
					},
				},
			},
		},
	}

	type args struct {
		fs       afero.Fs
		metaFile runtime.Object
	}

	type want struct {
		metaFile  metav1.Pkg
		readErr   error
		writerErr error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPriorCfgFile": {
			reason: "Should create file and read it back in without modification.",
			args: args{
				fs:       afero.NewMemMapFs(),
				metaFile: cfgMetaFile,
			},
			want: want{
				metaFile: cfgMetaFile,
			},
		},
		"NoPriorProviderFile": {
			reason: "Should create file and read it back in without modification.",
			args: args{
				fs:       afero.NewMemMapFs(),
				metaFile: providerMetaFile,
			},
			want: want{
				metaFile: providerMetaFile,
			},
		},
		"ErrReturnedWhenCannotWrite": {
			reason: "Should return an error if we cannot write to the fs.",
			args: args{
				fs:       afero.NewReadOnlyFs(afero.NewMemMapFs()),
				metaFile: providerMetaFile,
			},
			want: want{
				writerErr: syscall.EPERM,
				readErr:   errors.Wrap(errors.New("open /crossplane.yaml: file does not exist"), errMetaFileDoesNotExist),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// init workspace that is specific to this test
			ws := newTestWS(tc.args.fs)

			err := ws.writeMetaPkg(tc.args.metaFile)

			if diff := cmp.Diff(tc.want.writerErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// read meta file
			pkg, err := ws.readPkgMeta()
			if diff := cmp.Diff(tc.want.readErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.metaFile, pkg); diff != "" {
				t.Errorf("\n%s\nRWMetaFile(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestDependsOn(t *testing.T) {
	type args struct {
		fs       afero.Fs
		metaFile runtime.Object
	}

	type want struct {
		deps []v1beta1.Dependency
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleDependency": {
			reason: "Should return a slice with a single entry.",
			args: args{
				fs: afero.NewMemMapFs(),
				metaFile: &metav1.Configuration{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Configuration",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ConfigurationSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"MultipleDependencies": {
			reason: "Should return a slice with multiple entries.",
			args: args{
				fs: afero.NewMemMapFs(),
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
							DependsOn: []metav1.Dependency{
								{
									Configuration: pointer.String("crossplane/provider-gcp"),
									Version:       ">=v1.0.1",
								},
								{
									Provider: pointer.String("crossplane/provider-aws"),
									Version:  "v1.0.0",
								},
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-gcp",
						Type:        v1beta1.ConfigurationPackageType,
						Constraints: ">=v1.0.1",
					},
					{
						Package:     "crossplane/provider-aws",
						Type:        v1beta1.ProviderPackageType,
						Constraints: "v1.0.0",
					},
				},
			},
		},
		"NoDependencies": {
			reason: "Should return an empty slice.",
			args: args{
				fs: afero.NewMemMapFs(),
				metaFile: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1",
						Kind:       "Provider",
					},
					ObjectMeta: apimetav1.ObjectMeta{
						Name: "getting-started-with-aws",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{
							Crossplane: &metav1.CrossplaneConstraints{
								Version: ">=1.0.0-0",
							},
						},
					},
				},
			},
			want: want{
				deps: []v1beta1.Dependency{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// init workspace that is specific to this test
			ws := newTestWS(tc.args.fs)

			ws.writeMetaPkg(tc.args.metaFile)

			got, err := ws.DependsOn()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDependsOn(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.deps, got, cmpopts.SortSlices(func(i, j int) bool {
				return got[i].Package < got[j].Package
			})); diff != "" {
				t.Errorf("\n%s\nDependsOn(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

func newTestWS(fs afero.Fs) *Workspace {
	ws, _ := NewWorkspace(
		WithFS(fs),
		func(w *Workspace) {
			w.wd = func() (string, error) {
				return "/", nil
			}
		},
	)
	ws.Init()
	return ws
}
