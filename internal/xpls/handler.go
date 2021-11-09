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

package xpls

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/spf13/afero"
)

const (
	serverName = "xpls"

	defaultCacheDir = ".up/cache"
)

const (
	errParseWorkspace      = "failed to parse workspace"
	errParseSaveParameters = "failed to parse document save parameters"
	errValidateNodes       = "failed to validate nodes in workspace"
	errPublishDiagnostics  = "failed to publish diagnostics"
)

// HomeDirFn indicates the location of a user's home directory.
type HomeDirFn func() (string, error)

// A Handler handles LSP requests.
type Handler struct {
	root      string
	cacheDir  string
	cachePath string
	fs        afero.Fs
	ws        *Workspace
	home      HomeDirFn
	log       logging.Logger
}

// HandlerOpt modifies a handler.
type HandlerOpt func(h *Handler)

// WithFs sets the filesystem for the handler.
func WithFs(fs afero.Fs) HandlerOpt {
	return func(h *Handler) {
		h.fs = fs
	}
}

// WithCacheDir sets the cache directory relative to home directory.
func WithCacheDir(cache string) HandlerOpt {
	return func(h *Handler) {
		h.cacheDir = cache
	}
}

// WithLogger sets the logger for the handler.
func WithLogger(l logging.Logger) HandlerOpt {
	return func(h *Handler) {
		h.log = l
	}
}

// NewHandler constructs a new LSP handler,
func NewHandler(opts ...HandlerOpt) (*Handler, error) {
	h := &Handler{
		fs:       afero.NewOsFs(),
		home:     os.UserHomeDir,
		cacheDir: defaultCacheDir,
		log:      logging.NewNopLogger(),
	}
	for _, o := range opts {
		o(h)
	}
	// NOTE(hasheddan): using a HomeDirFn allows us to override home directory
	// location in testing, while still getting user directory from OS in normal
	// execution.
	home, err := h.home()
	if err != nil {
		return nil, err
	}
	h.cachePath = filepath.Join(home, h.cacheDir)
	return h, nil
}

// Handle handles LSP requests. It panics if we cannot initialize the workspace.
func (h *Handler) Handle(ctx context.Context, c *jsonrpc2.Conn, r *jsonrpc2.Request) { // nolint:gocyclo
	log := h.log.WithValues("request", r)
	switch r.Method {
	case "initialize":
		params := &lsp.InitializeParams{}
		if err := json.Unmarshal(*r.Params, params); err != nil {
			// If we can't understand the initialization parmaters panic because
			// future operations will not work.
			panic(err)
		}
		h.root = params.RootPath
		ws, err := NewWorkspace(h.root, h.cachePath)
		if err != nil {
			// If we can't construct the workspace panic because future
			// operations will not work.
			panic(err)
		}
		h.ws = ws
		if err := h.ws.Parse(); err != nil {
			log.Debug(errParseWorkspace, "error", err)
		}
		// TODO(hasheddan): perform initial validation on initialization.
		kind := lsp.TDSKIncremental
		if err := c.Reply(ctx, r.ID, lsp.InitializeResult{
			Capabilities: lsp.ServerCapabilities{
				TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
					Kind: &kind,
				},
			},
		}); err != nil {
			// If we fail to initialize the workspace we won't receive future
			// messages so we panic and try again on restart.
			panic(err)
		}
		return
	case "initialized":
		// NOTE(hasheddan): no need to respond when the client reports initialized.
		return
	case "textDocument/didSave":
		var params lsp.DidSaveTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			// If we can't parse the save parameters, log the error and skip
			// parsing.
			// TODO(hasheddan): surface this in diagnostics.
			h.log.Debug(errParseSaveParameters)
			break
		}
		if err := h.ws.Parse(); err != nil {
			// If we can't parse the workspace, log the error and skip validation.
			// TODO(hasheddan): surface this in diagnostics.
			h.log.Debug(errParseWorkspace, "error", err)
			break
		}
		// TODO(hasheddan): diagnostics should be cached and validation should
		// be performed selectively.
		diags, err := h.ws.Validate(AllNodes)
		if err != nil {
			h.log.Debug(errValidateNodes, "error", err)
			break
		}
		// TODO(hasheddan): we currently send all workspace diagnostics with
		// the text document URI from this save operation, meaning that
		// errors in other files are shown in it. We should first switch to
		// sending an individual set of diagnostics for each file with
		// errors, then move to maintaining a cache of diagnostics so that
		// we don't have to re-validate the entire workspace each time.
		if err := c.Notify(ctx, "textDocument/publishDiagnostics", &lsp.PublishDiagnosticsParams{
			URI:         params.TextDocument.URI,
			Diagnostics: diags,
		}); err != nil {
			h.log.Debug(errPublishDiagnostics, "error", err)
		}
	}
}
