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

	"github.com/upbound/up/internal/cloud"
)

const (
	listRowFormat = "%v\t%v\t%v\t%v\n"
)

// ListCmd list control planes in an account on Upbound Cloud.
type ListCmd struct{}

// Run executes the list command.
func (c *ListCmd) Run(kong *kong.Context, client *accounts.Client, cloudCtx *cloud.Context) error {
	cps, err := client.ListControlPlanes(context.Background(), cloudCtx.Account)
	if err != nil {
		return err
	}
	if len(cps) == 0 {
		return nil
	}
	w := printers.GetNewTabWriter(kong.Stdout)
	fmt.Fprintf(w, listRowFormat, "NAME", "ID", "SELF-HOSTED", "STATUS")
	for _, cp := range cps {
		fmt.Fprintf(w, listRowFormat, cp.ControlPlane.Name, cp.ControlPlane.ID, cp.ControlPlane.SelfHosted, cp.Status)
	}
	return w.Flush()
}
