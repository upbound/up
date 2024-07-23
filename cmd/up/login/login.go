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

package login

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/golang-jwt/jwt"
	"github.com/mdp/qrterminal/v3"
	"github.com/pkg/browser"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/userinfo"
	uphttp "github.com/upbound/up/internal/http"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	defaultTimeout = 30 * time.Second
	loginPath      = "/v1/login"

	webLogin            = "/login"
	issueEndpoint       = "/v1/issueTOTP"
	exchangeEndpoint    = "/v1/checkTOTP"
	totpDisplay         = "/cli/loginCode"
	loginResultEndpoint = "/cli/loginResult"

	errLoginFailed    = "unable to login"
	errReadBody       = "unable to read response body"
	errParseCookieFmt = "unable to parse session cookie: %s"
	errNoIDInToken    = "token is missing ID"
	errUpdateConfig   = "unable to update config file"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *LoginCmd) BeforeApply() error { //nolint:unparam
	c.stdin = os.Stdin
	c.prompter = input.NewPrompter()
	return nil
}

func (c *LoginCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	// NOTE(hasheddan): client timeout is handled with request context.
	// TODO(hasheddan): we can't use the typical up-sdk-go client here because
	// we need to read session cookie from body. We should add support in the
	// SDK so that we can be consistent across all commands.
	var tr http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: upCtx.InsecureSkipTLSVerify, //nolint:gosec
		},
	}
	if upCtx.WrapTransport != nil {
		tr = upCtx.WrapTransport(tr)
	}
	c.client = &http.Client{
		Transport: tr,
	}
	kongCtx.Bind(upCtx)
	if c.Token != "" {
		return nil
	}
	// Only prompt for password if username flag is explicitly passed
	if c.Password == "" && c.Username != "" {
		password, err := c.prompter.Prompt("Password", true)
		if err != nil {
			return err
		}
		c.Password = password
		return nil
	}
	u := *upCtx.Domain
	u.Host = "accounts." + u.Host
	c.accountsEndpoint = u

	return nil
}

// LoginCmd adds a user or token profile with session token to the up config
// file if a username is passed, but defaults to launching a web browser to authenticate with Upbound.
type LoginCmd struct {
	client   uphttp.Client
	stdin    io.Reader
	prompter input.Prompter

	Username string `short:"u" env:"UP_USER" xor:"identifier" help:"Username used to execute command."`
	Password string `short:"p" env:"UP_PASSWORD" help:"Password for specified user. '-' to read from stdin."`
	Token    string `short:"t" env:"UP_TOKEN" xor:"identifier" help:"Upbound API token (personal access token) used to execute command. '-' to read from stdin."`

	accountsEndpoint url.URL
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run executes the login command.
func (c *LoginCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error { // nolint:gocyclo
	// simple auth using explicit flags
	if c.Username != "" || c.Token != "" {
		return c.simpleAuth(ctx, p, upCtx)
	}

	// start webserver listening on port
	token := make(chan string, 1)
	redirect := make(chan string, 1)
	defer close(token)
	defer close(redirect)

	cb := callbackServer{
		token:    token,
		redirect: redirect,
	}
	err := cb.startServer()
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer cb.shutdownServer(ctx) //nolint:errcheck

	resultEP := c.accountsEndpoint
	resultEP.Path = loginResultEndpoint
	browser.Stderr = nil
	browser.Stdout = nil
	if err := browser.OpenURL(getEndpoint(c.accountsEndpoint, *upCtx.APIEndpoint, fmt.Sprintf("http://localhost:%d", cb.port))); err != nil {
		ep := getEndpoint(c.accountsEndpoint, *upCtx.APIEndpoint, "")
		qrterminal.Generate(ep, qrterminal.L, os.Stdout)
		fmt.Println("Could not open a browser!")
		fmt.Println("Please go to", ep, "and then enter code manually")
		// TODO(nullable-eth): Add a prompter with timeout?  Difficult to know when they actually
		// finished login to know when the TOTP would expire
		t, err := c.prompter.Prompt("Code", false)
		if err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
		token <- t
	}

	// wait for response on webserver or timeout
	timeout := uint(5)
	var t string
	select {
	case <-time.After(time.Duration(timeout) * time.Minute):
		break
	case t = <-token:
		break
	}

	if err := c.exchangeTokenForSession(ctx, p, upCtx, t); err != nil {
		resultEP.RawQuery = url.Values{
			"message": []string{err.Error()},
		}.Encode()
		redirect <- resultEP.String()
		return errors.Wrap(err, errLoginFailed)
	}
	redirect <- resultEP.String()
	return nil
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func setSession(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, res *http.Response, tokenType profile.TokenType, authID string) error {
	session, err := extractSession(res, upbound.CookieName)
	if err != nil {
		return err
	}

	// If profile name was not provided and no default exists, set name to 'default'.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = profile.DefaultName
	}

	// Re-initialize profile for this login.
	profile := profile.Profile{
		ID:        authID,
		TokenType: tokenType,
		// Set session early so that it can be used to fetch user info if
		// necessary.
		Session: session,
		// Carry over existing config.
		BaseConfig: upCtx.Profile.BaseConfig,
	}
	upCtx.Profile = profile

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
	p.Printfln("%s logged in", authID)
	return nil
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(username, token, password string) (*auth, profile.TokenType, error) {
	id, profType, err := parseID(username, token)
	if err != nil {
		return nil, "", err
	}
	if profType == profile.TokenTypeToken {
		password = token
	}
	return &auth{
		ID:       id,
		Password: password,
		Remember: true,
	}, profType, nil
}

// parseID gets a user ID by either parsing a token or returning the username.
func parseID(user, token string) (string, profile.TokenType, error) {
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
		return claims.Id, profile.TokenTypeToken, nil
	}
	return user, profile.TokenTypeUser, nil
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

func getEndpoint(account url.URL, api url.URL, local string) string {
	totp := local
	if local == "" {
		t := account
		t.Path = totpDisplay
		totp = t.String()
	}
	issueEP := api
	issueEP.Path = issueEndpoint
	issueEP.RawQuery = url.Values{
		"returnTo": []string{totp},
	}.Encode()

	loginEP := account
	loginEP.Path = webLogin
	loginEP.RawQuery = url.Values{
		"returnTo": []string{issueEP.String()},
	}.Encode()
	return loginEP.String()
}

func (c *LoginCmd) exchangeTokenForSession(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, t string) error {
	if t == "" {
		return errors.New("failed to receive callback from web login")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	e := *upCtx.APIEndpoint
	e.Path = exchangeEndpoint
	e.RawQuery = url.Values{
		"totp": []string{t},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close() // nolint:gosec,errcheck

	var user map[string]interface{} = make(map[string]interface{})
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return err
	}
	username, ok := user["username"].(string)
	if !ok {
		return errors.New("failed to get user details, code may have expired")
	}
	return setSession(ctx, p, upCtx, res, profile.TokenTypeUser, username)
}

type callbackServer struct {
	token    chan string
	redirect chan string
	port     int
	srv      *http.Server
}

func (cb *callbackServer) getResponse(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()["totp"]
	token := ""
	if len(v) == 1 {
		token = v[0]
	}

	// send the token
	cb.token <- token

	// wait for success or failure redirect
	rd := <-cb.redirect

	http.Redirect(w, r, rd, http.StatusSeeOther)
}

func (cb *callbackServer) shutdownServer(ctx context.Context) error {
	return cb.srv.Shutdown(ctx)
}

func (cb *callbackServer) startServer() (err error) {
	cb.port, err = cb.getPort()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", cb.getResponse)
	cb.srv = &http.Server{
		Handler:           mux,
		Addr:              fmt.Sprintf(":%d", cb.port),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	go func() error {
		if err := cb.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}() //nolint:errcheck

	return nil
}

func (cb *callbackServer) getPort() (int, error) {
	// Create a new server without specifying a port
	// which will result in an open port being chosen
	server, err := net.Listen("tcp", "localhost:0")

	// If there's an error it likely means no ports
	// are available or something else prevented finding
	// an open port
	if err != nil {
		return 0, err
	}
	defer server.Close() //nolint:errcheck

	// Split the host from the port
	_, portString, err := net.SplitHostPort(server.Addr().String())
	if err != nil {
		return 0, err
	}

	// Return the port as an int
	return strconv.Atoi(portString)
}

func (c *LoginCmd) simpleAuth(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
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
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
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
	return errors.Wrap(setSession(ctx, p, upCtx, res, profType, auth.ID), errLoginFailed)
}
