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

package dispatcher

import (
	"context"
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/golang/tools/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

const (
	errParseSaveParameters   = "failed to parse document save parameters"
	errParseChangeParameters = "failed to parse document change parameters"
)

// Server defines the set of LSP methods we currently support.
type Server interface {
	DidChange(context.Context, *protocol.DidChangeTextDocumentParams)
	DidOpen(context.Context, *protocol.DidOpenTextDocumentParams)
	DidSave(context.Context, *protocol.DidSaveTextDocumentParams)
	DidChangeWatchedFiles(context.Context, *protocol.DidChangeWatchedFilesParams)
	Initialize(context.Context, *jsonrpc2.Conn, jsonrpc2.ID, *protocol.InitializeParams)
}

// Dispatcher is responsible for routing JSONPPC request events to the
// appropriate place.
type Dispatcher struct {
	log logging.Logger
}

// New returns a new Dispatcher.
func New(opts ...Option) *Dispatcher {
	d := &Dispatcher{
		log: logging.NewNopLogger(),
	}

	for _, o := range opts {
		o(d)
	}

	return d
}

// Option provides a way to override default behavior of the Dispatcher.
type Option func(*Dispatcher)

// WithLogger overrides the default logging.Logger for the Dispatcher with the
// supplied logging.Logger.
func WithLogger(l logging.Logger) Option {
	return func(d *Dispatcher) {
		d.log = l
	}
}

// Dispatch dispatches the given JSONRPC request to the appropriate server function.
func (d *Dispatcher) Dispatch(ctx context.Context, server Server, conn *jsonrpc2.Conn, r *jsonrpc2.Request) { // nolint:gocyclo
	switch r.Method {
	case "initialize":
		var params protocol.InitializeParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			// If we can't understand the initialization parmaters panic because
			// future operations will not work.
			panic(err)
		}
		server.Initialize(ctx, conn, r.ID, &params)
		return
	case "initialized":
		// NOTE(hasheddan): no need to respond when the client reports initialized.
		return
	case "textDocument/didChange":
		var params protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			d.log.Debug(errParseChangeParameters)
			break
		}
		server.DidChange(ctx, &params)
		// publish diagnostics
		return
	case "textDocument/didOpen":
		var params protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			d.log.Debug(errParseSaveParameters)
			break
		}
		server.DidOpen(ctx, &params)
		return
	case "textDocument/didSave":
		var params protocol.DidSaveTextDocumentParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			// If we can't parse the save parameters, log the error and skip
			// parsing.
			// TODO(hasheddan): surface this in diagnostics.
			d.log.Debug(errParseSaveParameters)
			break
		}
		server.DidSave(ctx, &params)
		return
	case "workspace/didChangeWatchedFiles":
		var params protocol.DidChangeWatchedFilesParams
		if err := json.Unmarshal(*r.Params, &params); err != nil {
			d.log.Debug(errParseChangeParameters)
			break
		}

		server.DidChangeWatchedFiles(ctx, &params)
		return
	}
}
