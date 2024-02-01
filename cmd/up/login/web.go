// Copyright 2024Upbound Inc
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
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pkg/browser"
	"github.com/pterm/pterm"

	"github.com/mdp/qrterminal/v3"

	uphttp "github.com/upbound/up/internal/http"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	webLogin            = "/login"
	issueEndpoint       = "/v1/issueTOTP"
	exchangeEndpoint    = "/v1/checkTOTP"
	totpDisplay         = "/cli/loginCode"
	loginResultEndpoint = "/cli/loginResult"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *LoginWebCmd) BeforeApply() error { //nolint:unparam
	c.stdin = os.Stdin
	c.prompter = input.NewPrompter()
	return nil
}

func (c *LoginWebCmd) AfterApply(kongCtx *kong.Context) error {
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

	u := *upCtx.Domain
	u.Host = "accounts." + u.Host
	c.accountsEndpoint = u

	return nil
}

type LoginWebCmd struct {
	client   uphttp.Client
	stdin    io.Reader
	prompter input.Prompter

	accountsEndpoint url.URL
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run executes the list command.
func (c *LoginWebCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error { // nolint:gocyclo
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

func (c *LoginWebCmd) exchangeTokenForSession(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, t string) error {
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
	return setSession(ctx, p, upCtx, res, profile.User, username)
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
