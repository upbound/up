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
	"net/url"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/gitsources"
	"github.com/upbound/up/internal/upbound"
)

const (
	github      = "github.com"
	success     = "resultCode=success"
	redirectUri = "redirect_uri"
)

// createCmd creates a configuration on Upbound.
type createCmd struct {
	Name       string `arg:"" required:"" help:"Name of configuration."`
	Context    string `required:"" help:"Name of the GitHub account/org"`
	TemplateId string `required:"" help:"Name of the configuration template"`

	// The repo name is hidden. We'll set it to the name of the configuration, to match the UI's behavior
	Repo string `optional:"" hidden:"" help:"Name of the repo"`

	// Provider is hidden because it only has one allowed value (github).
	// We can expose it in the future.
	Provider string `hidden:"" default:"github" help:"Name of provider (e.g. github)"`

	Debug bool `hidden:"" help:"Debug auth workflow"`

	// NoPrompt is a temporary flag. Once we update MCP to allow redirections to the CLI
	// we'll eliminaite both this flag and the prompt
	NoPrompt bool `hidden:"" help:"Don't prompt user to hit return, but wait for callback"`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, cc *configurations.Client, gc *gitsources.Client, upCtx *upbound.Context) error {
	// By default, the repo name is the same as the configuration name
	// This matches the Console's behavior
	if c.Repo == "" {
		c.Repo = c.Name
	}
	if c.Provider == "" {
		c.Provider = string(configurations.ProviderGitHub)
	}

	// Step 1: Authorize and ionstall the GitHub app, if it needs to be installed.
	err := c.handleLogin(gc, upCtx)
	if err != nil {
		return err
	}

	// Step 2: Create the configuration
	return c.handleCreate(cc, upCtx)
}

// handleLogin uses the gitsources login API to authorize and install the GitHub app
func (c *createCmd) handleLogin(gc *gitsources.Client, upCtx *upbound.Context) error {
	r, err := gc.Login(context.Background())
	if err != nil {
		return err
	}

	if c.Debug {
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
			return c.continueLogin(r.RedirectURL, upCtx.Profile.Session)
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

// createLogin is called when the GitHub app hasn't been authorized or installed
// It does two things: It starts a webserver which will receive redirections,
// and it will open a web browser to start the authorization/installation process.
func (c *createCmd) continueLogin(url *url.URL, session string) error {
	s := authServer{
		debug:    c.Debug,
		session:  session,
		authDone: false,
	}
	u, _ := s.startHttpServer()
	if c.Debug {
		fmt.Printf("New auth redirect %s\n", u)
	}

	// The gitsources login API gave as a URL that we should send the user to.
	// However, it has an embedded redirect_uri parameter. We change that
	// parameter to be the web server we just pointed at. That way, we'll
	// know when the app has been authorized and installed so we can proceed
	// with the creation of the configuration.
	// We may be able to make server-side changes to simplify this in a future
	// iteration.
	values := url.Query()
	s.originalRedirect = values.Get(redirectUri)
	values.Del(redirectUri)
	values.Add(redirectUri, u)
	url.RawQuery = values.Encode()
	err := browser.OpenURL(url.String())
	if err != nil {
		return err
	}
	err = s.waitForFinalCallback(c.NoPrompt)
	return err
}

// handleCreate will create the configuration.
func (c *createCmd) handleCreate(cc *configurations.Client, upCtx *upbound.Context) error {
	params := configurations.ConfigurationCreateParameters{
		Name:       c.Name,
		Context:    c.Context,
		TemplateID: c.TemplateId,
		Provider:   configurations.Provider(c.Provider),
		Repo:       c.Repo,
	}
	_, err := cc.Create(context.Background(), upCtx.Account, &params)
	return err
}

// authServer is used to track state for the web server we create
type authServer struct {
	debug            bool
	session          string
	server           http.Server
	context          context.Context
	cancel           context.CancelFunc
	originalRedirect string
	code             string
	state            string
	authDone         bool
}

// startHttpServer creates the HTTP server we use to wait for the
// callbacks from Github
func (s *authServer) startHttpServer() (string, error) {
	s.context, s.cancel = context.WithCancel(context.Background())
	s.server = http.Server{
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	localUrl := fmt.Sprintf("http://127.0.0.1:%d/", listener.Addr().(*net.TCPAddr).Port)
	if s.debug {
		fmt.Printf("Starting web server: %s", localUrl)
	}
	http.HandleFunc("/", s.handleAuthCompletion)
	go func() {
		s.server.Serve(listener) //nolint:errcheck
	}()
	return localUrl, nil
}

// WaitForFinalCallback waits until the authorization and installation
// process is complete. We'll receive two callbacks (one for each)
//
// NOTE: In this first iteration, we can only receive the first callback.
// We need to update MCP API so that the user will be redirected to the CLI
// Therefore by default we'll just prompt the user to hit the return key.
// This is lame, but we'll iterate and get it right shortly.
func (s *authServer) waitForFinalCallback(noPrompt bool) error {
	if noPrompt {
		<-s.context.Done()
		err := s.server.Shutdown(context.Background())
		return err
	}
	fmt.Println("Hit enter once you've authorized and installed the GitHub app.")
	fmt.Println("(This is a temporary requirement that will go away soon.)")
	fmt.Scanln()
	return nil
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
	if s.debug {
		dump, _ := httputil.DumpRequest(r, true)
		fmt.Printf("Request:\n%s\n", string(dump))
	}

	if !s.authDone {
		// This is the first callback, received after the GitHub app has been authorized
		if s.debug {
			fmt.Printf("Received first callback\n")
			fmt.Printf("Original redirect: %s\n", s.originalRedirect)
		}

		// Redirect the user back to Upbound. Upbound wil redirect the user
		// to Github to install the application.
		values := r.URL.Query()
		s.code = values.Get("code")
		s.state = values.Get("state")
		newRedirect := fmt.Sprintf("%s?code=%s&state=%s", s.originalRedirect, s.code, s.state)
		if s.debug {
			fmt.Printf("New redirect: %s\n", newRedirect)
		}

		w.Header().Set("Set-Cookie", fmt.Sprintf("SID=%s", s.session))
		w.Header().Set("location", newRedirect)
		w.WriteHeader(http.StatusFound)
		return
	}

	// This is the second callback.
	// Today we won't receive the second callback, after the GitHub app has been installed.
	// We'll need to make server side changes to get the callback. The work is coming soon.
	fmt.Printf("Received second callback.\n")

	// Inform the other goroutine that the process is done.
	s.cancel()

}
