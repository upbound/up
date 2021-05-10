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

package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/config"
	uphttp "github.com/upbound/up/internal/http"
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
func (c *loginCmd) BeforeApply() error {
	// NOTE(hasheddan): client timeout is handled with request context.
	c.client = &http.Client{}
	return nil
}

// loginCmd adds a user or token profile with session token to the up config
// file.
type loginCmd struct {
	client uphttp.Client

	Password string `short:"p" env:"UP_PASSWORD" help:"Password for specified user."`
	Username string `short:"u" env:"UP_USER" xor:"identifier" help:"Username used to execute command."`
	Token    string `short:"t" env:"UP_TOKEN" xor:"identifier" help:"Token used to execute command."`
}

// Run executes the login command.
func (c *loginCmd) Run(cloudCtx *cloud.Context) error { // nolint:gocyclo
	// TODO(hasheddan): prompt for input if only username is supplied or
	// neither.
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
	cloudCtx.Endpoint.Path = loginPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudCtx.Endpoint.String(), bytes.NewReader(jsonStr))
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer res.Body.Close() // nolint:errcheck
	session, err := extractSession(res, cloud.CookieName)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	// If profile is not set, we assume operation on profile designated as
	// default in config.
	if cloudCtx.Profile == "" {
		cloudCtx.Profile = cloudCtx.Cfg.Cloud.Default
	}
	// If no default profile is specified, the profile is named `default`.
	if cloudCtx.Profile == "" {
		cloudCtx.Profile = defaultProfileName
	}
	// If no account is specified and profile type is user, set profile account
	// to user ID if not an email address. This is for convenience if a user is
	// using a personal account.
	if cloudCtx.Account == "" && profType == config.UserProfileType && !isEmail(auth.ID) {
		cloudCtx.Account = auth.ID
	}
	if err := cloudCtx.Cfg.AddOrUpdateCloudProfile(cloudCtx.Profile, config.Profile{
		ID:      auth.ID,
		Type:    profType,
		Session: session,
		Account: cloudCtx.Account,
	}); err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if len(cloudCtx.Cfg.Cloud.Profiles) == 1 {
		if err := cloudCtx.Cfg.SetDefaultCloudProfile(cloudCtx.Profile); err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
	}
	return errors.Wrap(cloudCtx.CfgSrc.UpdateConfig(cloudCtx.Cfg), errUpdateConfig)
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
	if password == "" {
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
