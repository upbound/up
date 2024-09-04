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

package organization

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthentication "k8s.io/client-go/pkg/apis/clientauthentication/v1"

	"github.com/upbound/up-sdk-go/service/auth"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// tokenCmd generates an org-scoped token for use with spaces.
type tokenCmd struct {
	Upbound upbound.Flags `embed:""`

	Name  string `arg:"" env:"ORGANIZATION" required:"" help:"Name of organization." predictor:"orgs"`
	Token string `short:"t" env:"UP_TOKEN" help:"Token used to execute command. Overrides the token present in the profile."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *tokenCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)
	return nil
}

// Run executes the token command.
func (c *tokenCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKAuthConfig()
	if err != nil {
		return err
	}

	sessionToken := c.Token
	if sessionToken == "" {
		sessionToken = upCtx.Profile.Session
	}

	client := auth.NewClient(cfg)
	orgToken, err := client.GetOrgScopedToken(ctx, c.Name, sessionToken)
	if err != nil {
		return err
	}

	exp := v1.NewTime(time.Now().Add(time.Duration(orgToken.ExpiresIn) * time.Second))

	creds := clientauthentication.ExecCredential{
		TypeMeta: v1.TypeMeta{
			Kind:       "ExecCredential",
			APIVersion: clientauthentication.SchemeGroupVersion.String(),
		},
		Status: &clientauthentication.ExecCredentialStatus{
			ExpirationTimestamp: &exp,
			Token:               orgToken.AccessToken,
		},
	}

	out, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	fmt.Print(string(out))
	return nil
}
