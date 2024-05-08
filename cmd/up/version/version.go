// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package version contains version cmd
package version

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/upbound/up/internal/version"
)

const (
	errGetCrossplaneVersion = "unable to get crossplane version"
	errGetSpacesVersion     = "unable to get spaces version"
)

const (
	versionUnknown         = "<unknown>"
	versionClientOutputFmt = "Client Version: %s"
	versionFullOutputFmt   = `Client Version: %s
Server Version: %s
Spaces Controller Version: %s
`
)

type Cmd struct {
	Client bool `env:"" help:"If true, shows client version only (no server required)." json:"client,omitempty"`
}

// BeforeApply sets default values and parses flags
func (c *Cmd) BeforeApply() error {
	flag.BoolVar(&c.Client, "client", false, "If true, shows client version only (no server required).")
	flag.Parse()
	return nil
}

func (c *Cmd) Help() string {
	return `
Options:
  --client=false:
  If true, shows client version only (no server required).

Usage:
  up version [flags] [options]
`
}

func (c *Cmd) Run(ctx context.Context) error {
	client := version.GetVersion()
	if c.Client {
		fmt.Printf(versionClientOutputFmt, client)
		return nil
	}

	vxp, err := FetchCrossplaneVersion(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrap(err, errGetCrossplaneVersion).Error())
	}
	if vxp == "" {
		vxp = versionUnknown
	}

	sc, err := FetchSpacesVersion(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrap(err, errGetSpacesVersion).Error())
	}
	if sc == "" {
		sc = versionUnknown
	}
	fmt.Printf(versionFullOutputFmt, client, vxp, sc)

	return nil
}
