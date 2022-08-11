// Copyright 2022 Upbound Inc
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

package config

import (
	"os"

	"github.com/pkg/errors"
	"github.com/upbound/up/internal/upbound"
)

type unsetCmd struct {
	Key string `arg:"" optional:"" help:"Configuration Key."`

	File *os.File `short:"f" help:"Configuration File. Must be in JSON format."`
}

func (c *unsetCmd) Run(upCtx *upbound.Context) error {
	if err := c.validateInput(); err != nil {
		return err
	}

	profile, _, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	cfg := map[string]any{
		c.Key: 0,
	}
	if c.File != nil {
		cfg, err = mapFromFile(c.File)
		if err != nil {
			return err
		}
	}

	if err := c.removeConfigs(upCtx, profile, cfg); err != nil {
		return err
	}
	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}

func (c *unsetCmd) validateInput() error {
	if c.Key != "" && c.File == nil {
		return nil
	}
	if c.Key == "" && c.File != nil {
		return nil
	}

	return errors.New(errOnlyKVFileXOR)
}

func (c *unsetCmd) removeConfigs(upCtx *upbound.Context, profile string, config map[string]any) error {
	for k := range config {
		if err := upCtx.Cfg.RemoveFromBaseConfig(profile, k); err != nil {
			return err
		}
	}
	return nil
}
