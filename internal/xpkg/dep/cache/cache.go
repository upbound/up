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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/dep/resolver/xpkg"
)

const (
	errFailedToAddEntry     = "failed to add entry to cache"
	errFailedToFindEntry    = "failed to find entry"
	errInvalidValueSupplied = "invalid value supplied"
)

// Local stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type Local struct {
	fs     afero.Fs
	home   config.HomeDirFn
	mu     sync.RWMutex
	root   string
	path   string
	pkgres XpkgResolver
}

// NewLocal creates a new LocalCache.
func NewLocal(opts ...Option) (*Local, error) {
	fs := afero.NewOsFs()
	l := &Local{
		fs:   fs,
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
	r, err := xpkg.NewResolver()
	if err != nil {
		return nil, err
	}

	l.root = root
	l.pkgres = r

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
func (c *Local) Get(k v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(image.FullTag(k))
	if err != nil {
		return nil, err
	}

	e, err := c.currentEntry(c.calculatePath(&t))
	if err != nil {
		return nil, err
	}

	return e.pkg, nil
}

// Store saves an image to the LocalCache. If a file currently
// exists at that location, we overwrite the current file.
func (c *Local) Store(k v1beta1.Dependency, v *xpkg.ParsedPackage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v == nil {
		return errors.New(errInvalidValueSupplied)
	}

	t, err := name.NewTag(image.FullTag(k))
	if err != nil {
		return err
	}

	path := c.calculatePath(&t)

	curr, err := c.currentEntry(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, errFailedToFindEntry)
	}

	e := c.newEntry(v)

	// clean the current entry
	if err := curr.Clean(); err != nil {
		return err
	}

	if err := c.add(e, path); err != nil {
		return err
	}

	return nil
}

// add the given entry to the supplied path (to)
func (c *Local) add(e *entry, to string) error {
	if err := c.ensureDirExists(filepath.Join(c.root, to)); err != nil {
		return err
	}

	e.setPath(to)

	if _, err := e.flush(); err != nil {
		return errors.Wrap(err, errFailedToAddEntry)
	}

	if err := e.setDigest(); err != nil {
		return errors.Wrap(err, errFailedToAddEntry)
	}

	return nil
}

// Clean removes all entries from the cache. Returns nil if the directory DNE.
func (c *Local) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.fs.RemoveAll(c.root)
}

// ensureDirExists ensures the target directory corresponding to the given path exists.
func (c *Local) ensureDirExists(path string) error {
	return c.fs.MkdirAll(path, os.ModePerm)
}

// calculatePath calculates the directory path from the given name.Tag following
// our convention.
// example:
//   tag: crossplane/provider-aws:v0.20.1-alpha
//   path: index.docker.io/crossplane/provider-aws@v0.20.1-alpha
func (c *Local) calculatePath(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@%s", tag.RepositoryStr(), tag.TagStr()),
	)
}
