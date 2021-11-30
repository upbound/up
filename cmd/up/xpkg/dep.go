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
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpkg/dep/workspace"
)

const (
	errMetaFileNotFound = "crossplane.yaml file not found in current directory"
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

	r := image.NewResolver()

	m, err := manager.New(
		manager.WithCache(cache),
		manager.WithResolver(r),
	)

	if err != nil {
		return err
	}

	c.m = m
	c.c = cache

	ws, err := workspace.New(workspace.WithFS(fs))
	if err != nil {
		return err
	}
	c.ws = ws

	// workaround interfaces not being bindable ref: https://github.com/alecthomas/kong/issues/48
	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// depCmd manages crossplane dependencies.
type depCmd struct {
	c  *cache.Local
	m  *manager.Manager
	ws *workspace.Workspace

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

	if c.Package != "" {
		return c.userSuppliedDep(ctx)
	}

	return c.metaSuppliedDeps(ctx)
}

func (c *depCmd) userSuppliedDep(ctx context.Context) error {
	// exit early check if we were supplied an invalid package string
	_, err := xpkg.ValidDep(c.Package)
	if err != nil {
		return err
	}

	d := dep.New(c.Package)

	ud, _, err := c.m.AddAll(ctx, d)
	if err != nil {
		return err
	}

	if c.ws.MetaExists() {
		// crossplane.yaml file exists in the workspace, upsert the new dependency
		if err := c.ws.Upsert(ud); err != nil {
			return err
		}
	}

	return nil
}

func (c *depCmd) metaSuppliedDeps(ctx context.Context) error {
	if !c.ws.MetaExists() {
		return errors.New(errMetaFileNotFound)
	}

	deps, err := c.ws.DependsOn()
	if err != nil {
		return err
	}

	for _, d := range deps {
		if _, _, err := c.m.AddAll(ctx, d); err != nil {
			return err
		}
	}

	return nil
}
