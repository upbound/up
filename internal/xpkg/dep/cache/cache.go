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
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
)

const (
	errFailedToAddEntry     = "failed to add entry to cache"
	errFailedToFindEntry    = "failed to find entry"
	errInvalidValueSupplied = "invalid value supplied"
	errInvalidVersion       = "invalid version found"
)

// Local stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type Local struct {
	fs     afero.Fs
	home   config.HomeDirFn
	mu     sync.RWMutex
	root   string
	path   string
	pkgres XpkgMarshaler
}

// XpkgMarshaler defines the API contract for working marshaling
// xpkg.ParsedPackage's from a directory.
type XpkgMarshaler interface {
	FromDir(afero.Fs, string, string, string) (*xpkg.ParsedPackage, error)
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

	// TODO(@tnthornton) this is probably not what we want. Otherwise if
	// a cachedir is specified outside of the home path, we'll install
	// in the home path anyways
	root, err := filepath.Abs(filepath.Join(home, l.path))
	if err != nil {
		return nil, err
	}
	r, err := xpkg.NewMarshaler()
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

// Root returns the calculated root of the cache.
func (c *Local) Root() string {
	return c.root
}

// Get retrieves an image from the LocalCache.
func (c *Local) Get(k v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(image.FullTag(k))
	if err != nil {
		return nil, err
	}

	e, err := c.currentEntry(calculatePath(&t), t.RegistryStr(), t.RepositoryStr())
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

	path := calculatePath(&t)

	curr, err := c.currentEntry(path, t.RegistryStr(), t.RepositoryStr())
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

// Versions returns a slice of versions that exist in the cache for the given
// package.
func (c *Local) Versions(k v1beta1.Dependency) ([]string, error) {
	t, err := name.NewTag(k.Package)
	if err != nil {
		return nil, err
	}

	glob := calculateVersionsGlob(&t)

	matches, err := afero.Glob(c.fs, filepath.Join(c.root, glob))
	if err != nil {
		return nil, err
	}
	vers := make([]string, 0)
	for _, m := range matches {
		ver := strings.Split(m, "@")
		if len(ver) != 2 {
			return nil, errors.New(errInvalidVersion)
		}
		vers = append(vers, ver[1])
	}

	return vers, nil
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

	return nil
}

// Clean removes all entries from the cache. Returns nil if the directory DNE.
func (c *Local) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	files, err := afero.ReadDir(c.fs, c.root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, f := range files {
		if err := c.fs.RemoveAll(filepath.Join(c.root, f.Name())); err != nil {
			return err
		}
	}
	return nil
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
func calculatePath(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@%s", tag.RepositoryStr(), tag.TagStr()),
	)
}

func calculateVersionsGlob(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@*", tag.RepositoryStr()),
	)
}
