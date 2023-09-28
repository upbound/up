// Copyright 2023 Upbound Inc
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

package configuration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/gitsources"
	"github.com/upbound/up/internal/upbound"
)

const (
	github  = "github.com"
	success = "resultCode=success"
)

// createCmd creates a configuration on Upbound.
type createCmd struct {
	Name       string `arg:"" required:"" help:"Name of configuration."`
	Context    string `required:"" help:"Name of the GitHub account/org"`
	TemplateId string `required:"" help:"Name of the configuration template" predictor:"templates"`
	Private    bool   `default:"false" help:"Whether the Github repo should be created as private. (Default: false)"`

	// The repo name is hidden. We'll set it to the name of the configuration, to match the UI's behavior
	Repo string `optional:"" hidden:"" help:"Name of the repo"`

	// Provider is hidden because it only has one allowed value (github).
	// We can expose it in the future.
	Provider string `hidden:"" default:"github" help:"Name of provider (e.g. github)"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, cc *configurations.Client, gc *gitsources.Client, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpaces() {
		return fmt.Errorf("create is not supported for Spaces profile %q", upCtx.ProfileName)
	}

	// By default, the repo name is the same as the configuration name
	// This matches the Console's behavior
	if c.Repo == "" {
		c.Repo = c.Name
	}
	if c.Provider == "" {
		c.Provider = string(configurations.ProviderGitHub)
	}

	// Step 1: Authorize and install the GitHub app, if it needs to be installed.
	err := c.handleLogin(gc, upCtx)
	if err != nil {
		return err
	}

	// Step 2: Create the configuration
	return c.handleCreate(cc, upCtx)
}

// handleLogin uses the gitsources login API to authorize and install the GitHub app
func (c *createCmd) handleLogin(gc *gitsources.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	s := authServer{
		debugLevel: upCtx.DebugLevel,
		session:    upCtx.Profile.Session,
		authDone:   false,
	}
	port, err := s.startHttpServer()
	if err != nil {
		return err
	}
	defer s.shutdown() //nolint:errcheck

	r, err := gc.Login(context.Background(), port)
	if err != nil {
		return err
	}

	if s.debugLevel > 0 {
		fmt.Printf("Login response:\nStatus: %d\nAuth Redirect: %s\n\n",
			r.StatusCode,
			r.RedirectURL.String())
	}

	if r.StatusCode >= 300 && r.StatusCode < 400 {
		if r.RedirectURL == nil {
			return errors.New("Cannot find redirection URL.")
		}
		hostname := r.RedirectURL.Hostname()
		if hostname == "" {
			return errors.New("Cannot find redirection host.")
		}
		switch {
		case strings.HasSuffix(hostname, github):
			// Case 1: The user hasn't yet authorized or installed the GitHub app
			// We'll open a web browser and wait for the process to complete
			err := browser.OpenURL(r.RedirectURL.String())
			if err != nil {
				return err
			}
			s.WaitForCallback()
			return nil
		case strings.Contains(r.RedirectURL.String(), success):
			// Case 2: The GitHub app has already been authorized and installed
			fmt.Printf("No need to authorize Upbound Github App: already authorized\n")
			return nil
		default:
			// Case 3: The first time this API is called after de-authorizing the GitHub app,
			// we get an error. It disappears shortly after calling the API.
			// TODO: Try again, maybe with a pause? This is expected to be a rare case,
			// so it's low priority to make it smoother.
			return errors.New("The Upbound GitHub App is not authorized and cannot be authorized now. Try again later.\n")
		}
	}

	return errors.New("Failed to be redirected")
}

// handleCreate will create the configuration.
func (c *createCmd) handleCreate(cc *configurations.Client, upCtx *upbound.Context) error {
	params := configurations.ConfigurationCreateParameters{
		Name:       c.Name,
		Context:    c.Context,
		TemplateID: c.TemplateId,
		Provider:   configurations.Provider(c.Provider),
		Repo:       c.Repo,
		Private:    c.Private,
	}
	_, err := cc.Create(context.Background(), upCtx.Account, &params)
	return err
}

// authServer is used to track state for the web server we create
type authServer struct {
	debugLevel int
	session    string
	server     http.Server
	context    context.Context
	cancel     context.CancelFunc
	authDone   bool
}

// startHttpServer creates the HTTP server we use to wait for the
// callbacks from Github
func (s *authServer) startHttpServer() (int, error) {
	var port = 0
	s.context, s.cancel = context.WithCancel(context.Background())
	s.server = http.Server{
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return port, err
	}
	port = listener.Addr().(*net.TCPAddr).Port
	localUrl := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if s.debugLevel > 0 {
		fmt.Printf("Starting web server: %s", localUrl)
	}
	http.HandleFunc("/", s.handleAuthCompletion)
	go func() {
		s.server.Serve(listener) //nolint:errcheck
	}()
	return port, nil
}

// WaitForCallback waits until the authorization and installation
// process is complete. We'll receive two callbacks (one for each)
//
// NOTE: In this first iteration, we can only receive the first callback.
// We need to update MCP API so that the user will be redirected to the CLI
// Therefore by default we'll just prompt the user to hit the return key.
// This is lame, but we'll iterate and get it right shortly.
func (s *authServer) WaitForCallback() {
	<-s.context.Done()
}

func (s *authServer) shutdown() error {
	err := s.server.Shutdown(context.Background())
	return err
}

// handleAuthCompletion is an HTTP handler, and it receives the callbacks
// from GitHub. We'll receive two callbacks: one after authorizing the GitHub app
// and one after installing the GitHub app.
//
// NOTE: In this first iteration, we can only receive the first callback.
// We need to update MCP API so that the user will be redirected to the CLI
// Therefore by default we'll just prompt the user to hit the return key.
// This is lame, but we'll iterate and get it right.
func (s *authServer) handleAuthCompletion(w http.ResponseWriter, r *http.Request) {
	if s.debugLevel > 0 {
		dump, _ := httputil.DumpRequest(r, true)
		fmt.Printf("Request:\n%s\n", string(dump))
	}

	fmt.Fprintf(w, "You have authorized and install the GitHub app. You can close this window now.\n")
	fmt.Printf("The GitHub app has been authorized and installed.\n")

	// Inform the other goroutine that the process is done.
	s.cancel()
}
