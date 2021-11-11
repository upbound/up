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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
)

// A Cache caches OCI images.
type Cache interface {
	Get(v1beta1.Dependency) (v1.Image, error)
	Store(v1beta1.Dependency, v1.Image) error
	Delete(v1beta1.Dependency) error

	Clean() error
}

// Local stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type Local struct {
	fs   afero.Fs
	home config.HomeDirFn
	mu   sync.RWMutex
	root string
	path string
}

// NewLocal creates a new LocalCache.
func NewLocal(opts ...Option) (*Local, error) {
	l := &Local{
		fs:   afero.NewOsFs(),
		home: os.UserHomeDir,
	}

	for _, o := range opts {
		o(l)
	}

	home, err := l.home()
	if err != nil {
		return nil, err
	}

	root, err := filepath.Abs(filepath.Join(home, l.path))
	if err != nil {
		return nil, err
	}
	l.root = root
	return l, nil
}

// Option represents an option that can be applied to Local
type Option func(*Local)

// WithFS defines the filesystem that is configured for Local
func WithFS(fs afero.Fs) Option {
	return func(l *Local) {
		l.fs = fs
	}
}

// WithRoot defines the root of the cache
func WithRoot(root string) Option {
	return func(l *Local) {
		// in the event ~/cache/dir is passed in trim ~/ to avoid $HOME/~/cache/dir
		l.path = strings.TrimPrefix(root, "~/")
	}
}

// Get retrieves an image from the LocalCache.
func (c *Local) Get(k v1beta1.Dependency) (v1.Image, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(dep.ImgTag(k))
	if err != nil {
		return nil, err
	}

	dir := c.dir(&t)

	l, err := c.currentFile(dir)
	if os.IsNotExist(err) {
		return nil, err
	}

	return tarball.Image(
		c.opener(l),
		&t,
	)
}

// Store saves an image to the LocalCache. If a file currently
// exists at that location, we overwrite the current file.
func (c *Local) Store(k v1beta1.Dependency, v v1.Image) error { // nolint:gocyclo
	c.mu.Lock()
	defer c.mu.Unlock()
	imgTag := dep.ImgTag(k)

	ref, err := name.ParseReference(imgTag)
	if err != nil {
		return err
	}

	t, err := name.NewTag(imgTag)
	if err != nil {
		return err
	}

	d := c.dir(&t)

	// make sure the directory structure exists for this entry
	if err := c.fs.MkdirAll(d, os.ModePerm); err != nil {
		return err
	}

	h, err := v.Digest()
	if err != nil {
		return err
	}

	// does an older file exist?
	cur, err := c.currentFile(d)
	if err != nil {
		return err
	}

	// clean up old file if one exists
	if cur != "" {
		if err = c.fs.Remove(cur); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	cf, err := c.fs.Create(c.newFile(d, h.String()))
	if err != nil {
		return err
	}

	if err := tarball.Write(ref, v, cf); err != nil {
		return err
	}
	return cf.Close()
}

// Delete removes an image from the ImageCache.
func (c *Local) Delete(k v1beta1.Dependency) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	t, err := name.NewTag(dep.ImgTag(k))
	if err != nil {
		return err
	}

	l, err := c.currentFile(c.dir(&t))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	err = c.fs.Remove(l)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Clean removes all entries from the cache. Returns nil if the directory DNE.
func (c *Local) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.fs.RemoveAll(c.root)
}

func (c *Local) opener(path string) tarball.Opener {
	return func() (io.ReadCloser, error) {
		return c.fs.Open(path)
	}
}

// example tag: index.docker.io/crossplane/provider-aws:v0.20.1-alpha
func (c *Local) dir(tag *name.Tag) string {
	return filepath.Join(
		c.root,
		tag.RegistryStr(),
		fmt.Sprintf("%s@%s", tag.RepositoryStr(), tag.TagStr()),
	)
}

func (c *Local) currentFile(path string) (string, error) {
	files, err := afero.ReadDir(c.fs, path)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return filepath.Join(path, files[0].Name()), nil
	}
	return "", nil
}

func (c *Local) newFile(path, digest string) string {
	return filepath.Join(path, digest) + xpkg.XpkgExtension
}
