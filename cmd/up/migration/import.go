// Copyright 2023 Upbound Inc
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

package migration

import (
	"context"
	"k8s.io/client-go/rest"
	"net/url"
	"strings"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/importer"
)

type importCmd struct {
	Input string `short:"i" help:"Input archive path." default:"xp-state.tar.gz"`

	UnpauseAfterImport bool `help:"Unpause all managed resources after importing." default:"true"`
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	cfg := migCtx.Kubeconfig

	/*if !isMCP(cfg) {
		return errors.New("not a managed control plane, import not supported!")
	}*/

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	i := importer.NewControlPlaneStateImporter(dynamicClient, discoveryClient, mapper, importer.Options{
		InputArchive: c.Input,

		UnpauseAfterImport: true,
	})
	if err = i.Import(ctx); err != nil {
		return err
	}

	return nil
}

func isMCP(cfg *rest.Config) bool {
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return false
	}
	return (strings.HasPrefix(u.Path, "/v1/controlplanes") || strings.HasPrefix(u.Path, "/v1/controlPlanes")) && strings.HasSuffix(u.Path, "/k8s")
}
