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
	"context"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/pkg/errors"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
)

const (
	errBuildMetaScheme   = "failed to build meta scheme for manager"
	errBuildObjectScheme = "failed to build object scheme for manager"

	errNotMeta = "meta type is not a package"
)

// Manager defines a dependency Manager
type Manager struct {
	c *cache.Local
	f dep.Fetcher
	p *parser.PackageParser
	r *dep.Resolver
}

// New returns a new Manager
func New(opts ...Option) (*Manager, error) {
	m := &Manager{}

	c, err := cache.NewLocal()
	if err != nil {
		return nil, err
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return nil, errors.New(errBuildMetaScheme)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	m.c = c
	m.f = dep.NewLocalFetcher()
	m.p = parser.New(metaScheme, objScheme)
	m.r = dep.NewResolver()

	for _, o := range opts {
		o(m)
	}

	return m, nil
}

// Option modifies the Manager.
type Option func(*Manager)

// WithCache sets the supplied cache.Local on the Manager.
func WithCache(c *cache.Local) Option {
	return func(m *Manager) {
		m.c = c
	}
}

// WithFetcher sets the supplied dep.LocalFetcher on the Manager.
func WithFetcher(f dep.Fetcher) Option {
	return func(m *Manager) {
		m.f = f
	}
}

// WithResolver sets the supplied dep.Resolver on the Manager.
func WithResolver(r *dep.Resolver) Option {
	return func(m *Manager) {
		m.r = r
	}
}

// Resolve resolves the given package as well as it's transitive dependencies.
// If storage is successful, the resolved dependency is returned, errors
// otherwise.
func (m *Manager) Resolve(ctx context.Context, d v1beta1.Dependency) (v1beta1.Dependency, error) {
	ud := v1beta1.Dependency{}

	e, err := m.retrieveEntry(ctx, d)
	if err != nil {
		return ud, nil
	}

	// recursively resolve all transitive dependencies
	// currently assumes we have something from
	if err := m.resolveAllDeps(ctx, e); err != nil {
		return ud, err
	}

	ud.Type = e.Type()
	ud.Package = d.Package
	ud.Constraints = e.Version()

	return ud, nil
}

// resolveAllDeps recursively resolves the transitive dependencies
// for a given Entry.
func (m *Manager) resolveAllDeps(ctx context.Context, e *cache.Entry) error {

	pkg, ok := xpkg.TryConvertToPkg(e.Meta(), &metav1.Provider{}, &metav1.Configuration{})
	if !ok {
		return errors.New(errNotMeta)
	}

	if len(pkg.GetDependencies()) == 0 {
		// no remaining dependencies to resolve
		return nil
	}

	for _, d := range pkg.GetDependencies() {
		cd := ConvertToV1beta1(d)

		e, err := m.retrieveEntry(ctx, cd)
		if err != nil {
			return err
		}

		if err := m.resolveAllDeps(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) storePkg(ctx context.Context, d v1beta1.Dependency) (*cache.Entry, error) {
	// this is expensive
	i, err := m.r.ResolveImage(ctx, d)
	if err != nil {
		return nil, err
	}

	// add xpkg to cache
	e, err := m.c.Store(d, i)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (m *Manager) retrieveEntry(ctx context.Context, d v1beta1.Dependency) (*cache.Entry, error) {
	// resolve version prior to Get
	if err := m.finalizeDepVersion(ctx, &d); err != nil {
		return nil, err
	}

	e, err := m.c.Get(d)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		// root dependency does not yet exist in cache, store it
		e, err = m.storePkg(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		// check if digest is different from what we have locally
		digest, err := m.r.ResolveDigest(ctx, d)
		if err != nil {
			return nil, err
		}

		if e.Digest() != digest {
			// digest is different, update what we have
			e, err = m.storePkg(ctx, d)
			if err != nil {
				return nil, err
			}
		}
	}

	return e, nil
}

// finalizeDepVersion sets the resolved tag version on the supplied v1beta1.Dependency.
func (m *Manager) finalizeDepVersion(ctx context.Context, d *v1beta1.Dependency) error {
	// determine the version (using resolver) to use based on the supplied constraints
	v, err := m.r.ResolveTag(ctx, *d)
	if err != nil {
		return err
	}

	d.Constraints = v
	return nil
}
