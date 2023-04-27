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

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/golang-jwt/jwt"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/userinfo"
	"github.com/upbound/up/internal/config"
	uphttp "github.com/upbound/up/internal/http"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultProfileName = "default"
	loginPath          = "/v1/login"

	errLoginFailed    = "unable to login"
	errReadBody       = "unable to read response body"
	errParseCookieFmt = "unable to parse session cookie: %s"
	errNoUserOrToken  = "either username or token must be provided"
	errNoIDInToken    = "token is missing ID"
	errUpdateConfig   = "unable to update config file"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *loginCmd) BeforeApply() error { //nolint:unparam
	c.stdin = os.Stdin
	c.prompter = input.NewPrompter()
	return nil
}

func (c *loginCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	// NOTE(hasheddan): client timeout is handled with request context.
	// TODO(hasheddan): we can't use the typical up-sdk-go client here because
	// we need to read session cookie from body. We should add support in the
	// SDK so that we can be consistent across all commands.
	c.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: upCtx.InsecureSkipTLSVerify, //nolint:gosec
			},
		},
	}
	kongCtx.Bind(upCtx)
	if c.Token != "" {
		return nil
	}
	if c.Username == "" {
		username, err := c.prompter.Prompt("Username", false)
		if err != nil {
			return err
		}
		c.Username = username
	}
	if c.Password == "" {
		password, err := c.prompter.Prompt("Password", true)
		if err != nil {
			return err
		}
		c.Password = password
	}
	return nil
}

// loginCmd adds a user or token profile with session token to the up config
// file.
type loginCmd struct {
	client   uphttp.Client
	stdin    io.Reader
	prompter input.Prompter

	Username string `short:"u" env:"UP_USER" xor:"identifier" help:"Username used to execute command."`
	Password string `short:"p" env:"UP_PASSWORD" help:"Password for specified user. '-' to read from stdin."`
	Token    string `short:"t" env:"UP_TOKEN" xor:"identifier" help:"Token used to execute command. '-' to read from stdin."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run executes the login command.
func (c *loginCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error { // nolint:gocyclo
	if c.Token == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Token = strings.TrimSpace(string(b))
	}
	if c.Password == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Password = strings.TrimSpace(string(b))
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	auth, profType, err := constructAuth(c.Username, c.Token, c.Password)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	jsonStr, err := json.Marshal(auth)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	loginEndpoint := *upCtx.APIEndpoint
	loginEndpoint.Path = loginPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint.String(), bytes.NewReader(jsonStr))
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer res.Body.Close() // nolint:gosec,errcheck
	session, err := extractSession(res, upbound.CookieName)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}

	// Set session early so that it can be used to fetch user info if necessary.
	upCtx.Profile.Session = session

	// If the default account is not set, the user's personal account is used.
	if upCtx.Account == "" {
		conf, err := upCtx.BuildSDKConfig()
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
		info, err := userinfo.NewClient(conf).Get(ctx)
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
		upCtx.Account = info.User.Username
	}

	// If profile name was not provided and no default exists, set name to 'default'.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = defaultProfileName
	}

	upCtx.Profile.ID = auth.ID
	upCtx.Profile.Type = profType
	upCtx.Profile.Account = upCtx.Account

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := upCtx.Cfg.SetDefaultUpboundProfile(upCtx.ProfileName); err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}
	p.Printfln("%s logged in", auth.ID)
	return nil
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(username, token, password string) (*auth, config.ProfileType, error) {
	if username == "" && token == "" {
		return nil, "", errors.New(errNoUserOrToken)
	}
	id, profType, err := parseID(username, token)
	if err != nil {
		return nil, "", err
	}
	if profType == config.TokenProfileType {
		password = token
	}
	return &auth{
		ID:       id,
		Password: password,
		Remember: true,
	}, profType, nil
}

// parseID gets a user ID by either parsing a token or returning the username.
func parseID(user, token string) (string, config.ProfileType, error) {
	if token != "" {
		p := jwt.Parser{}
		claims := &jwt.StandardClaims{}
		_, _, err := p.ParseUnverified(token, claims)
		if err != nil {
			return "", "", err
		}
		if claims.Id == "" {
			return "", "", errors.New(errNoIDInToken)
		}
		return claims.Id, config.TokenProfileType, nil
	}
	return user, config.UserProfileType, nil
}

// extractSession extracts the specified cookie from an HTTP response. The
// caller is responsible for closing the response body.
func extractSession(res *http.Response, cookieName string) (string, error) {
	for _, cook := range res.Cookies() {
		if cook.Name == cookieName {
			return cook.Value, nil
		}
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, errReadBody)
	}
	return "", errors.Errorf(errParseCookieFmt, string(b))
}

// isEmail determines if the specified username is an email address.
func isEmail(user string) bool {
	return strings.Contains(user, "@")
}
