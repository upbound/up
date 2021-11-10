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

package xpkg

import (
	"context"

	"github.com/alecthomas/kong"
	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/dep"
	"github.com/upbound/up/internal/dep/cache"
	"github.com/upbound/up/internal/xpkg"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *depCmd) AfterApply(kongCtx *kong.Context) error {
	ctx := context.Background()
	fs := afero.NewOsFs()

	cache, err := cache.NewLocal(
		cache.WithFS(fs),
		cache.WithRoot(c.CacheDir),
	)
	if err != nil {
		return err
	}

	c.c = cache
	c.f = dep.NewLocalFetcher()
	c.r = dep.NewResolver(dep.WithFetcher(c.f))

	ws, err := dep.NewWorkspace(dep.WithFS(fs))
	if err != nil {
		return err
	}
	c.ws = ws

	// don't resolve the given dependency if we want to clean the cache
	if !c.CleanCache {
		// exit early check if we were supplied an invalid package string
		_, err := xpkg.ValidDep(c.Package)
		if err != nil {
			return err
		}

		c.d = dep.New(c.Package)

		// determine the version (using resolver) to use based on the supplied constraints
		v, err := c.r.ResolveTag(ctx, c.d)
		if err != nil {
			return err
		}

		c.d.Version = v
	}

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// depCmd manages crossplane dependencies.
type depCmd struct {
	c  *cache.Local
	d  v1.Dependency
	f  *dep.LocalFetcher
	r  *dep.Resolver
	ws *dep.Workspace

	CacheDir   string `short:"d" help:"Directory used for caching package images." default:"~/.up/cache/" env:"CACHE_DIR"`
	CleanCache bool   `short:"c" help:"Clean dep cache."`

	Package string `arg:"" optional:"" help:"Package to be added."`
}

// Run executes the dep command.
func (c *depCmd) Run(ctx context.Context) error {
	// no need to do anything else if clean cache was called.

	// TODO (@tnthornton) this feels a little out of place here. We should
	// consider adding a separate command for doing this.
	if c.CleanCache {
		return c.c.Clean()
	}

	i, err := c.r.ResolveImage(ctx, c.d)
	if err != nil {
		return err
	}

	// add xpkg to cache
	if err := c.c.Store(c.d, i); err != nil {
		return err
	}

	// init workspace
	if err := c.ws.Init(); err != nil {
		return err
	}

	// if a meta file exists in the ws, update it
	if c.ws.MetaExists() {
		// `crossplane.yaml file exists in the workspace, upsert the new dependency`
		if err := c.ws.Upsert(c.d); err != nil {
			return err
		}
	}

	return nil
}
