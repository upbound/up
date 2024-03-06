// Copyright 2024 Upbound Inc
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

package template

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Cmd struct {
}

func (c *Cmd) Help() string {
	return `
Usage:
    tview-template [options]

The 'tview-template' brings happiness.`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *Cmd) BeforeApply() error {
	return nil
}

func (c *Cmd) Run(ctx context.Context) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	client, err := rest.TransportFor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	app := NewApp("upbound tview-template", client, cfg.Host)
	return app.Run(ctx)
}
