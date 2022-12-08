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
	"strconv"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/repositories"
	repos "github.com/upbound/up-sdk-go/service/repositories"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists repositories in an account on Upbound.
type listCmd struct{}

var fieldNames = []string{"NAME", "TYPE", "PUBLIC", "UPDATED"}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	rList, err := rc.List(context.Background(), upCtx.Account, common.WithSize(maxItems))
	if err != nil {
		return err
	}
	if len(rList.Repositories) == 0 {
		p.Printfln("No repositories found in %s", upCtx.Account)
		return nil
	}
	return printer.Print(rList.Repositories, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	r := obj.(repos.Repository)

	rt := "unknown"
	if r.Type != nil {
		rt = string(*r.Type)
	}
	u := "n/a"
	if r.UpdatedAt != nil {
		u = duration.HumanDuration(time.Since(*r.UpdatedAt))
	}
	return []string{r.Name, rt, strconv.FormatBool(r.Public), u}
}
