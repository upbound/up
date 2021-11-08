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

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/dep"
	"github.com/upbound/up/internal/xpkg"
)

// A Cache caches OCI images.
type Cache interface {
	Get(Key) (v1.Image, error)
	Store(Key, v1.Image) error
	Delete(Key) error

	Clean() error
}

// Local stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type Local struct {
	fs afero.Fs

	root string
	mu   sync.RWMutex
}

// NewLocal creates a new LocalCache.
func NewLocal(fs afero.Fs, root string) *Local {
	return &Local{
		fs:   fs,
		root: resolveHome(root),
	}
}

func resolveHome(root string) string {
	tilde := "~/"

	if strings.HasPrefix(root, tilde) {
		rootsplit := strings.Split(root, tilde)
		home, _ := os.UserHomeDir()
		return filepath.Join(home, rootsplit[1])
	}

	return root
}

// Key represents a cache key that is composed of a package's image tag
// and its name.
type Key struct {
	imgTag string
}

// NewKey returns a new cache key
func NewKey(d metav1.Dependency) Key {
	return Key{
		imgTag: dep.ImgTag(d),
	}
}

// Get retrieves an image from the LocalCache.
func (c *Local) Get(k Key) (v1.Image, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(k.imgTag)
	if err != nil {
		return nil, err
	}

	d := c.dir(&t)

	l, err := c.currentFile(d)
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
func (c *Local) Store(k Key, v v1.Image) error { // nolint:gocyclo
	c.mu.Lock()
	defer c.mu.Unlock()
	ref, err := name.ParseReference(k.imgTag)
	if err != nil {
		return err
	}

	t, err := name.NewTag(k.imgTag)
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
func (c *Local) Delete(k Key) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	t, err := name.NewTag(k.imgTag)
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
