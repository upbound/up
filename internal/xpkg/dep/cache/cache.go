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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
)

const (
	errFailedToAddEntry  = "failed to add entry to cache"
	errFailedToFindEntry = "failed to find entry"

	errOpenPackageStream = "failed to open package stream file"
)

// A Cache caches OCI images.
type Cache interface {
	Get(v1beta1.Dependency) (*Entry, error)
	GetPkgType(v1beta1.Dependency) (string, error)
	Store(v1beta1.Dependency, v1.Image) error

	Clean() error
}

// Local stores and retrieves OCI images in a filesystem-backed cache in a
// thread-safe manner.
type Local struct {
	fs     afero.Fs
	home   config.HomeDirFn
	mu     sync.RWMutex
	root   string
	path   string
	parser *parser.PackageParser
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

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return nil, errors.New(errBuildMetaScheme)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	l.parser = parser.New(metaScheme, objScheme)

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
func (c *Local) Get(k v1beta1.Dependency) (*Entry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, err := name.NewTag(dep.ImgTag(k))
	if err != nil {
		return nil, err
	}

	return c.CurrentEntry(c.resolvePath(&t))
}

// GetPkgType retrieves the package type for the given dependency's meta file
func (c *Local) GetPkgType(k v1beta1.Dependency) (string, error) {
	e, err := c.Get(k)
	if err != nil {
		return "", err
	}

	return string(e.Type()), nil
}

// Store saves an image to the LocalCache. If a file currently
// exists at that location, we overwrite the current file.
func (c *Local) Store(k v1beta1.Dependency, v v1.Image) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	t, err := name.NewTag(dep.ImgTag(k))
	if err != nil {
		return err
	}

	path := c.resolvePath(&t)

	curr, err := c.CurrentEntry(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, errFailedToFindEntry)
	}

	e, err := c.NewEntry(v)
	if err != nil {
		return err
	}

	// TODO(@tnthornton) we can add a check to skip persisting if we already
	// have the latest version of the dependency stored.

	// clean the current entry
	if err := curr.Clean(); err != nil {
		return err
	}

	return c.add(e, path)
}

// add the given entry to the supplied path (to)
func (c *Local) add(e *Entry, to string) error {
	if err := c.ensureDirExists(filepath.Join(c.root, to)); err != nil {
		return err
	}

	e.setPath(to)

	if _, _, _, err := e.flush(); err != nil {
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

// resolvePath resolves the given image tag to a directory path following our
// convention.
// example:
//   tag: crossplane/provider-aws:v0.20.1-alpha
//   path: index.docker.io/crossplane/provider-aws@v0.20.1-alpha
func (c *Local) resolvePath(tag *name.Tag) string {
	return filepath.Join(
		tag.RegistryStr(),
		fmt.Sprintf("%s@%s", tag.RepositoryStr(), tag.TagStr()),
	)
}
