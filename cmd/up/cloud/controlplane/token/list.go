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

package token

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"k8s.io/cli-runtime/pkg/printers"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/upbound"
)

const (
	listRowFormat = "%v\t%v\t%v\n"

	resNoTokens = "No tokens found for control plane."
)

// listCmd lists tokens for a control plane on Upbound Cloud.
type listCmd struct {
	ID uuid.UUID `arg:"" name:"control-plane-ID" required:"" help:"ID of control plane."`
}

// Run executes the list command.
func (c *listCmd) Run(kong *kong.Context, client *cp.Client, upCtx *upbound.Context) error {
	res, err := client.GetTokens(context.Background(), c.ID)
	if err != nil {
		return err
	}
	if res == nil || len(res.DataSet) == 0 {
		fmt.Fprintf(kong.Stdout, "%s\n", resNoTokens)
		return nil
	}
	w := printers.GetNewTabWriter(kong.Stdout)
	fmt.Fprintf(w, listRowFormat, "ID", "NAME", "CREATED")
	for _, token := range res.DataSet {
		fmt.Fprintf(w, listRowFormat, token.ID, token.AttributeSet["name"], token.Meta["createdAt"])
	}
	return w.Flush()
}
