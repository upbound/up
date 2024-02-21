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

package token

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/upbound"
)

const (
	ErrUserAccount      = "robots are not currently supported for user accounts"
	ErrMultipleRobotFmt = "found multiple robots with name %s in %s"
	ErrMultipleTokenFmt = "found multiple tokens with name %s for robot %s in %s"
	ErrFindRobotFmt     = "could not find robot %s in %s"
	ErrFindTokenFmt     = "could not find token %s for robot %s in %s"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(tokens.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing robot tokens.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a token for the robot."`
	Delete deleteCmd `cmd:"" help:"Delete a token for the robot."`
	List   listCmd   `cmd:"" help:"List the tokens for the robot."`
	Get    getCmd    `cmd:"" help:"Get a token for the robot."`
}
