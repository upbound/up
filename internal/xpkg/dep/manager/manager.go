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

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/dep/resolver/xpkg"
	xpkgparser "github.com/upbound/up/internal/xpkg/parser"
)

// Manager defines a dependency Manager
type Manager struct {
	c  *cache.Local
	f  image.Fetcher
	r  *image.Resolver
	rx *xpkg.Resolver
}

// New returns a new Manager
func New(opts ...Option) (*Manager, error) {
	m := &Manager{}

	c, err := cache.NewLocal()
	if err != nil {
		return nil, err
	}

	p, err := xpkgparser.New()
	if err != nil {
		return nil, err
	}

	m.c = c
	m.f = image.NewLocalFetcher()
	m.rx = xpkg.NewResolver(xpkg.WithParser(p))

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
func WithFetcher(f image.Fetcher) Option {
	return func(m *Manager) {
		m.f = f
	}
}

// WithResolver sets the supplied dep.Resolver on the Manager.
func WithResolver(r *image.Resolver) Option {
	return func(m *Manager) {
		m.r = r
	}
}

// Resolve resolves the given package as well as it's transitive dependencies.
// If storage is successful, the resolved dependency is returned, errors
// otherwise.
func (m *Manager) Resolve(ctx context.Context, d v1beta1.Dependency) (v1beta1.Dependency, error) {
	ud := v1beta1.Dependency{}

	e, err := m.retrievePkg(ctx, d)
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
func (m *Manager) resolveAllDeps(ctx context.Context, p *xpkg.ParsedPackage) error {

	if len(p.Dependencies()) == 0 {
		// no remaining dependencies to resolve
		return nil
	}

	for _, d := range p.Dependencies() {
		e, err := m.retrievePkg(ctx, d)
		if err != nil {
			return err
		}

		if err := m.resolveAllDeps(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) storePkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// this is expensive
	t, i, err := m.r.ResolveImage(ctx, d)
	if err != nil {
		return nil, err
	}

	p, err := m.rx.FromImage(d.Package, t, i)
	if err != nil {
		return nil, err
	}

	// add xpkg to cache
	err = m.c.Store(d, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (m *Manager) retrievePkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// resolve version prior to Get
	if err := m.finalizeDepVersion(ctx, &d); err != nil {
		return nil, err
	}

	p, err := m.c.Get(d)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		// root dependency does not yet exist in cache, store it
		p, err = m.storePkg(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		// check if digest is different from what we have locally
		digest, err := m.r.ResolveDigest(ctx, d)
		if err != nil {
			return nil, err
		}

		if p.Digest() != digest {
			// digest is different, update what we have
			p, err = m.storePkg(ctx, d)
			if err != nil {
				return nil, err
			}
		}
	}

	return p, nil
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
