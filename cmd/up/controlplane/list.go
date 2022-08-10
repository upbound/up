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

package controlplane

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/upbound"
)

const (
	listRowFormat = "%v\t%v\t%v\n"

	maxItems = 100
)

// ListCmd list control planes in an account on Upbound.
type ListCmd struct{}

// Run executes the list command.
func (c *ListCmd) Run(experimental bool, kongCtx *kong.Context, ac *accounts.Client, cc *cp.Client, upCtx *upbound.Context) error {
	var cps []cp.ControlPlaneResponse
	var err error
	if experimental {
		// TODO(hasheddan): we currently just max out single page size, but we
		// may opt to support limiting page size and iterating through pages via
		// flags in the future.
		cpList, err := cc.List(context.Background(), upCtx.Account, common.WithSize(maxItems))
		if err != nil {
			return err
		}
		if len(cpList.ControlPlanes) == 0 {
			return nil
		}
		cps = cpList.ControlPlanes
	} else {
		cps, err = ac.ListControlPlanes(context.Background(), upCtx.Account)
		if err != nil {
			return err
		}
		if len(cps) == 0 {
			return nil
		}
	}
	w := printers.GetNewTabWriter(kongCtx.Stdout)
	fmt.Fprintf(w, listRowFormat, "NAME", "ID", "STATUS")
	for _, cp := range cps {
		fmt.Fprintf(w, listRowFormat, cp.ControlPlane.Name, cp.ControlPlane.ID, cp.Status)
	}
	return w.Flush()
}
