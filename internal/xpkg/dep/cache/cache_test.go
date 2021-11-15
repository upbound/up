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

package cache

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"

	ociname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
)

var (
	providerAws = "crossplane/provider-aws"

	testProviderMetaYaml = "testdata/provider_meta.yaml"
	testProviderPkgYaml  = "testdata/provider_package.yaml"
)

func TestGet(t *testing.T) {
	fs := afero.NewMemMapFs()
	i := newPackageImage(testProviderPkgYaml)

	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot("/cache"),
		// override HomeDirFn
		rootIsHome,
	)

	e, _ := cache.NewEntry(i)

	cache.add(e, "index.docker.io/crossplane/provider-aws@v0.20.1-alpha")

	type args struct {
		cache *Local
		key   v1beta1.Dependency
	}

	type want struct {
		err error
		val *Entry
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha",
				},
			},
			want: want{
				val: &Entry{
					sha: digest(i),
				},
			},
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha1",
				},
			},
			want: want{
				err: &os.PathError{Op: "open", Path: "/cache/index.docker.io/crossplane/provider-aws@v0.20.1-alpha1", Err: afero.ErrFileNotFound},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, err := tc.args.cache.Get(tc.args.key)

			if tc.want.val != nil {
				if diff := cmp.Diff(tc.want.val.Digest(), e.Digest()); diff != "" {
					t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetPkgType(t *testing.T) {
	cache, _ := NewLocal(
		WithFS(afero.NewMemMapFs()),
		WithRoot("/cache"),
		rootIsHome,
	)

	e, _ := cache.NewEntry(newPackageImage(testProviderPkgYaml))
	cache.add(e, "index.docker.io/crossplane/provider-aws@v0.20.1-alpha")

	type args struct {
		cache *Local
		key   v1beta1.Dependency
	}

	type want struct {
		err     error
		pkgType v1beta1.PackageType
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha",
				},
			},
			want: want{
				pkgType: v1beta1.ProviderPackageType,
			},
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha1",
				},
			},
			want: want{
				err: &os.PathError{Op: "open", Path: "/cache/index.docker.io/crossplane/provider-aws@v0.20.1-alpha1", Err: afero.ErrFileNotFound},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pt, err := tc.args.cache.GetPkgType(tc.args.key)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(string(tc.want.pkgType), pt); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStore(t *testing.T) {
	dep1 := v1beta1.Dependency{
		Package:     "crossplane/exist-xpkg",
		Type:        v1beta1.ProviderPackageType,
		Constraints: "latest",
	}

	dep2 := v1beta1.Dependency{
		Package:     "crossplane/dep2-xpkg",
		Type:        v1beta1.ProviderPackageType,
		Constraints: "latest",
	}

	type setup struct {
		dep v1beta1.Dependency
		img v1.Image
	}

	type args struct {
		cache *Local
		dep   v1beta1.Dependency
		img   v1.Image
		setup *setup
	}

	type want struct {
		metaSha        [32]byte
		pkgDigest      string
		cacheFileCount int
		err            error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should have crossplane.yaml and the expected number of files if successful.",
			args: args{
				cache: newLocalCache(
					WithFS(afero.NewMemMapFs()),
					WithRoot("/tmp/cache"),
					rootIsHome,
				),
				dep: dep1,
				img: newPackageImage(testProviderPkgYaml),
			},
			want: want{
				metaSha:        sha256.Sum256(metaTestData()),
				pkgDigest:      digest(newPackageImage(testProviderPkgYaml)),
				cacheFileCount: 4,
			},
		},
		"AddSecondDependency": {
			reason: "Should not return an error if we have multiple packages in cache.",
			args: args{
				cache: newLocalCache(
					WithFS(afero.NewMemMapFs()),
					WithRoot("/tmp/cache"),
					rootIsHome,
				),
				dep: dep2,
				img: newPackageImage(testProviderPkgYaml),
				setup: &setup{
					dep: dep1,
					img: newPackageImage(testProviderPkgYaml),
				},
			},
			want: want{
				metaSha:        sha256.Sum256(metaTestData()),
				pkgDigest:      digest(newPackageImage(testProviderPkgYaml)),
				cacheFileCount: 8,
			},
		},
		"Replace": {
			reason: "Should not return an error if we're replacing the pre-existing image.",
			args: args{
				cache: newLocalCache(
					WithFS(afero.NewMemMapFs()),
					WithRoot("/tmp/cache"),
					rootIsHome,
				),
				dep: dep1,
				img: newPackageImage(testProviderPkgYaml),
				setup: &setup{
					dep: dep1,
					img: newPackageImage(testProviderPkgYaml),
				},
			},
			want: want{
				metaSha:        sha256.Sum256(metaTestData()),
				pkgDigest:      digest(newPackageImage(testProviderPkgYaml)),
				cacheFileCount: 4,
			},
		},
		"ErrFailedCreate": {
			reason: "Should return an error if file creation fails.",
			args: args{
				cache: newLocalCache(
					WithFS(afero.NewReadOnlyFs(afero.NewMemMapFs())),
					WithRoot("/tmp/cache"),
					rootIsHome,
				),
				dep: dep1,
				img: newPackageImage(testProviderPkgYaml),
			},
			want: want{
				err:            syscall.EPERM,
				cacheFileCount: 0,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.args.setup != nil {
				// establish a pre-existing entry
				_, err := tc.args.cache.Store(tc.args.setup.dep, tc.args.setup.img)
				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}

			_, err := tc.args.cache.Store(tc.args.dep, tc.args.img)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if tc.want.err == nil {

				e, _ := tc.args.cache.Get(tc.args.dep)

				b, _ := yaml.Marshal(e.Meta())
				got := sha256.Sum256(b)

				if diff := cmp.Diff(tc.want.metaSha, got); diff != "" {
					t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.pkgDigest, e.Digest()); diff != "" {
					t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.cacheFileCount, cacheFileCnt(tc.args.cache.fs, tc.args.cache.root)); diff != "" {
					t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestClean(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot("~/.up/cache"),
	)
	readOnlyCache, _ := NewLocal(
		WithFS(afero.NewReadOnlyFs(fs)),
		WithRoot("~/.up/cache"),
	)

	type args struct {
		cache *Local
	}

	type want struct {
		preCleanFileCnt  int
		postCleanFileCnt int
		err              error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should not return an error if cache was cleaned.",
			args: args{
				cache: cache,
			},
			want: want{
				preCleanFileCnt:  8,
				postCleanFileCnt: 0,
			},
		},
		"ErrFailedClean": {
			reason: "Should return an error if we failed to clean the cache.",
			args: args{
				cache: readOnlyCache,
			},
			want: want{
				preCleanFileCnt:  8,
				postCleanFileCnt: 8,
				err:              syscall.EPERM,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// add a few entries to cache
			e1, _ := cache.NewEntry(newPackageImage(testProviderPkgYaml))
			cache.add(e1, "index.docker.io/crossplane/provider-aws@v0.20.1-alpha")

			e2, _ := cache.NewEntry(newPackageImage(testProviderPkgYaml))
			cache.add(e2, "index.docker.io/crossplane/provider-gcp@v0.14.2")

			c := cacheFileCnt(fs, tc.args.cache.root)

			if diff := cmp.Diff(tc.want.preCleanFileCnt, c); diff != "" {
				t.Errorf("\n%s\nClean(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			err := tc.args.cache.Clean()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nClean(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			c = cacheFileCnt(fs, tc.args.cache.root)

			if diff := cmp.Diff(tc.want.postCleanFileCnt, c); diff != "" {
				t.Errorf("\n%s\nClean(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tag1, _ := ociname.NewTag("crossplane/provider-aws:v0.20.1-alpha")
	tag2, _ := ociname.NewTag("gcr.io/crossplane/provider-gcp:v1.0.0")
	tag3, _ := ociname.NewTag("registry.upbound.io/examples-aws/getting-started:v0.14.0-240.g6a7366f")

	c, _ := NewLocal(
		WithFS(afero.NewMemMapFs()),
		WithRoot("/cache"),
		rootIsHome,
	)

	type args struct {
		tag *ociname.Tag
	}
	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"SuccessDockerIO": {
			reason: "Should return formatted cache path with packageName as filename.",
			args: args{
				tag: &tag1,
			},
			want: "index.docker.io/crossplane/provider-aws@v0.20.1-alpha",
		},
		"SuccessGCR": {
			reason: "Should return formatted cache path with packageName as filename.",
			args: args{
				tag: &tag2,
			},
			want: "gcr.io/crossplane/provider-gcp@v1.0.0",
		},
		"SuccessUpboundRegistry": {
			reason: "Should return formatted cache path with packageName as filename.",
			args: args{
				tag: &tag3,
			},
			want: "registry.upbound.io/examples-aws/getting-started@v0.14.0-240.g6a7366f",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := c.resolvePath(tc.args.tag)

			if diff := cmp.Diff(tc.want, d); diff != "" {
				t.Errorf("\n%s\nResolvePath(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func newPackageImage(path string) v1.Image {
	pack, _ := os.Open(path)

	info, _ := pack.Stat()

	buf := new(bytes.Buffer)

	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: info.Size(),
	}
	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, pack)
	_ = tw.Close()
	packLayer, _ := tarball.LayerFromReader(buf)
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	return packImg
}

func cacheFileCnt(fs afero.Fs, dir string) int {
	var cnt int
	afero.Walk(fs, dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				cnt++
			}
			return nil
		})

	return cnt
}

func digest(i v1.Image) string {
	h, _ := i.Digest()
	return h.String()
}

func metaTestData() []byte {
	meta, _ := os.Open(testProviderMetaYaml)

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, meta)

	return buf.Bytes()
}

var (
	rootIsHome = func(l *Local) {
		l.home = func() (string, error) {
			return "/", nil
		}
	}

	newLocalCache = func(opts ...Option) *Local {
		c, _ := NewLocal(opts...)
		return c
	}
)
