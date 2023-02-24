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

package repository

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	repos "github.com/upbound/up-sdk-go/service/repositories"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single repo.
type getCmd struct {
	Name string `arg:"" required:"" help:"Name of repo." predictor:"repos"`
}

// Run executes the get command.
func (c *getCmd) Run(printer upterm.ObjectPrinter, rc *repos.Client, upCtx *upbound.Context) error {
	repo, err := rc.Get(context.Background(), upCtx.Account, c.Name)
	if err != nil {
		return err
	}

	// We convert to a list so we can match the output of the list command
	repoList := repos.RepositoryListResponse{
		Repositories: []repos.Repository{repo.Repository},
	}
	return printer.Print(repoList.Repositories, fieldNames, extractFields)
}
