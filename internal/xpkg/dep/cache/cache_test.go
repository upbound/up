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
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/upbound/up/internal/xpkg/dep"

	ociname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

var (
	providerAws = "crossplane/provider-aws"
)

func TestGet(t *testing.T) {
	fs := afero.NewMemMapFs()

	cRoot := "/cache"

	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot(cRoot),
		// override HomeDirFn
		rootIsHome,
	)

	manualAddEntry(fs, entry{
		root:  cRoot,
		path:  "index.docker.io/crossplane/provider-aws@v0.20.1-alpha",
		image: empty.Image,
		tag:   "crossplane/provider-aws:v0.20.1-alpha",
	})

	type args struct {
		cache *Local
		key   v1beta1.Dependency
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
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
			want: &os.PathError{Op: "open", Path: "/cache/index.docker.io/crossplane/provider-aws@v0.20.1-alpha1", Err: afero.ErrFileNotFound},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.args.cache.Get(tc.args.key)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStore(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot("/cache"),
		rootIsHome,
	)

	readOnlyCache, _ := NewLocal(
		WithFS(afero.NewReadOnlyFs(fs)),
		WithRoot("/new-cache/"),
		rootIsHome,
	)

	existsd := v1beta1.Dependency{
		Package:     "crossplane/exist-xpkg",
		Type:        v1beta1.ProviderPackageType,
		Constraints: "latest",
	}

	type args struct {
		cache *Local
		key   v1beta1.Dependency
		val   v1.Image
	}
	type want struct {
		err      error
		numFiles int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				cache: cache,
				key:   existsd,
				val:   empty.Image,
			},
			want: want{
				numFiles: 1,
			},
		},
		"Replace": {
			reason: "Should not return an error if we're replacing the pre-existing image.",
			args: args{
				cache: cache,
				key:   existsd,
				val:   newEmptyImage([]byte("stuff and things")),
			},
			want: want{
				numFiles: 1,
			},
		},
		"ErrFailedCreate": {
			reason: "Should return an error if file creation fails.",
			args: args{
				cache: readOnlyCache,
				key:   existsd,
				val:   empty.Image,
			},
			want: want{
				err:      syscall.EPERM,
				numFiles: 0,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Store(tc.args.key, tc.args.val)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			tag, _ := ociname.NewTag(dep.ImgTag(tc.args.key))
			files, _ := afero.ReadDir(fs, tc.args.cache.dir(&tag))

			if diff := cmp.Diff(tc.want.numFiles, len(files)); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot("/cache"),
		rootIsHome,
	)

	readOnlyCache, _ := NewLocal(
		WithFS(afero.NewReadOnlyFs(fs)),
		WithRoot("/cache"),
		rootIsHome,
	)

	manualAddEntry(fs, entry{
		root:  "/cache",
		path:  "index.docker.io/crossplane/provider-aws@v0.20.1-alpha",
		image: empty.Image,
		tag:   "crossplane/provider-aws:v0.20.1-alpha",
	})

	type args struct {
		cache *Local
		key   v1beta1.Dependency
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package is deleted at path.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha",
				},
			},
		},
		"SuccessNotExist": {
			reason: "Should not return an error if package does not exist.",
			args: args{
				cache: cache,
				key: v1beta1.Dependency{
					Package: "not-exists/not-exists",
				},
			},
		},
		"ErrFailedDelete": {
			reason: "Should return an error if file deletion fails.",
			args: args{
				cache: readOnlyCache,
				key: v1beta1.Dependency{
					Package:     providerAws,
					Constraints: "v0.20.1-alpha",
				},
			},
			want: syscall.EPERM,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Delete(tc.args.key)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want err, +got err:\n%s", tc.reason, diff)
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
		WithRoot("/cache"),
		rootIsHome,
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
				preCleanFileCnt:  2,
				postCleanFileCnt: 0,
			},
		},
		"ErrFailedClean": {
			reason: "Should return an error if we failed to clean the cache.",
			args: args{
				cache: readOnlyCache,
			},
			want: want{
				preCleanFileCnt:  2,
				postCleanFileCnt: 2,
				err:              syscall.EPERM,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			// add a few entries to cache
			manualAddEntry(fs, entry{
				root:  tc.args.cache.root,
				path:  "index.docker.io/crossplane/provider-aws@v0.20.1-alpha",
				image: empty.Image,
				tag:   "crossplane/provider-aws:v0.20.1-alpha",
			})

			manualAddEntry(fs, entry{
				root:  tc.args.cache.root,
				path:  "index.docker.io/crossplane/provider-gcp@v0.14.2",
				image: empty.Image,
				tag:   "crossplane/provider-gcp:v0.14.2",
			})

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

func TestDir(t *testing.T) {
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
			want: "/cache/index.docker.io/crossplane/provider-aws@v0.20.1-alpha",
		},
		"SuccessGCR": {
			reason: "Should return formatted cache path with packageName as filename.",
			args: args{
				tag: &tag2,
			},
			want: "/cache/gcr.io/crossplane/provider-gcp@v1.0.0",
		},
		"SuccessUpboundRegistry": {
			reason: "Should return formatted cache path with packageName as filename.",
			args: args{
				tag: &tag3,
			},
			want: "/cache/registry.upbound.io/examples-aws/getting-started@v0.14.0-240.g6a7366f",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := c.dir(tc.args.tag)

			if diff := cmp.Diff(tc.want, d); diff != "" {
				t.Errorf("\n%s\nBuildPath(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func newEmptyImage(contents []byte) v1.Image {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)

	w.Write(contents)

	layer, _ := tarball.LayerFromReader(buf)
	newImg, _ := mutate.AppendLayers(empty.Image, layer)

	return newImg
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

type entry struct {
	root  string
	path  string
	image v1.Image
	tag   string
}

func manualAddEntry(fs afero.Fs, e entry) {
	d, _ := e.image.Digest()
	basepath := filepath.Join(e.root, e.path)
	fs.MkdirAll(basepath, os.ModePerm)

	path := filepath.Join(basepath, fmt.Sprintf("%s.xpkg", d.String()))
	cf, _ := fs.Create(path)
	tag, _ := ociname.NewTag(e.tag)
	tarball.Write(tag, empty.Image, cf)
}

var rootIsHome = func(l *Local) {
	l.home = func() (string, error) {
		return "/", nil
	}
}
