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

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/golang/tools/lsp/protocol"
)

const (
	serverName      = "xpls"
	defaultCacheDir = ".up/cache"

	errParseSaveParameters   = "failed to parse document save parameters"
	errParseChangeParameters = "failed to parse document change parameters"
	errPublishDiagnostics    = "failed to publish diagnostics"
)

// HomeDirFn indicates the location of a user's home directory.
type HomeDirFn func() (string, error)

// A Handler handles LSP requests.
type Handler struct {
	cacheDir string
	dispatch *Dispatcher
	fs       afero.Fs
	home     HomeDirFn
	log      logging.Logger
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
	h.dispatch = NewDispatcher(h.log, h.cacheDir)
	return h, nil
}

// Handle handles LSP requests. It panics if we cannot initialize the workspace.
func (h *Handler) Handle(ctx context.Context, c *jsonrpc2.Conn, r *jsonrpc2.Request) { // nolint:gocyclo
	switch r.Method {
	case "initialize":
		var params lsp.InitializeParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			// If we can't understand the initialization parmaters panic because
			// future operations will not work.
			panic(err)
		}
		reply := h.dispatch.Initialize(ctx, params)

		if err := c.Reply(ctx, r.ID, reply); err != nil {
			// If we fail to initialize the workspace we won't receive future
			// messages so we panic and try again on restart.
			panic(err)
		}
		return
	case "initialized":
		// NOTE(hasheddan): no need to respond when the client reports initialized.
		return
	case "textDocument/didOpen":
		var params lsp.DidOpenTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			h.log.Debug(errParseSaveParameters)
			break
		}
		diags := h.dispatch.DidOpen(ctx, params)
		if diags == nil {
			// an error occurred while processing diagnostics, skip for now.
			break
		}
		if err := c.Notify(ctx, "textDocument/publishDiagnostics", diags); err != nil {
			h.log.Debug(errPublishDiagnostics, "error", err)
		}
	case "textDocument/didSave":
		var params lsp.DidSaveTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			// If we can't parse the save parameters, log the error and skip
			// parsing.
			// TODO(hasheddan): surface this in diagnostics.
			h.log.Debug(errParseSaveParameters)
			break
		}

		diags := h.dispatch.DidSave(ctx, params)
		if diags == nil {
			// an error occurred while processing diagnostics, skip for now.
			break
		}

		// TODO(hasheddan): we currently send all workspace diagnostics with
		// the text document URI from this save operation, meaning that
		// errors in other files are shown in it. We should first switch to
		// sending an individual set of diagnostics for each file with
		// errors, then move to maintaining a cache of diagnostics so that
		// we don't have to re-validate the entire workspace each time.
		if err := c.Notify(ctx, "textDocument/publishDiagnostics", diags); err != nil {
			h.log.Debug(errPublishDiagnostics, "error", err)
		}
	case "textDocument/didChange":
		// need to keep track of a snapshot of the workspace inmem, with filename -> doc string
		// on change, grab doc string from snapshot, stitch in change (in order)
		var params protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			h.log.Debug(errParseChangeParameters)
			break
		}

		diags := h.dispatch.DidChange(ctx, params)
		if diags == nil {
			// an error occurred while processing diagnostics, skip for now.
			break
		}

		if err := c.Notify(ctx, "textDocument/publishDiagnostics", diags); err != nil {
			h.log.Debug(errPublishDiagnostics, "error", err)
		}
	}
}
