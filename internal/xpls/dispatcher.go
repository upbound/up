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
	"fmt"
	"path/filepath"

	"github.com/golang/tools/lsp/protocol"
	"github.com/sourcegraph/go-lsp"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/xpkg"
)

var (
	// kind describes how text synchronization works.
	kind = lsp.TDSKIncremental
)

const (
	errParseWorkspace = "failed to parse workspace"
	errValidateNodes  = "failed to validate nodes in workspace"
	errLoadValidators = "failed to load validators"
)

// Dispatcher --
type Dispatcher struct {
	root      string
	cacheRoot string

	ws  *Workspace
	log logging.Logger
}

// NewDispatcher returns a new Dispatcher instance.
func NewDispatcher(log logging.Logger, cacheRoot string) *Dispatcher {
	return &Dispatcher{
		cacheRoot: cacheRoot,
		log:       log,
	}
}

// Initialize handles initialize events.
func (d *Dispatcher) Initialize(ctx context.Context, params lsp.InitializeParams) *lsp.InitializeResult {
	d.root = params.RootPath
	ws, err := NewWorkspace(d.root, d.cacheRoot)
	if err != nil {
		panic(err)
	}

	d.ws = ws

	// TODO(@tnthornton) this is a slow operation
	if err := d.ws.LoadCacheValidators(); err != nil {
		// TODO(@tnthornton) while at first glance, panicing here makes sense
		// i.e. we simply can't function correctly, it's unclear to me if
		// that's the correct choice from an end user UX perspective.
		panic(err)
	}

	if err := d.ws.LoadValidators(d.root); err != nil {
		// If we can't load validators panic because we won't be able to
		// perform validation.
		panic(err)
	}

	if err := d.ws.Parse(); err != nil {
		d.log.Debug(errParseWorkspace, "error", err)
		panic(err)
	}

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Kind: &kind,
			},
		},
	}
}

// DidChange handles didChange events.
func (d *Dispatcher) DidChange(ctx context.Context, params protocol.DidChangeTextDocumentParams) *lsp.PublishDiagnosticsParams {
	uri := params.TextDocument.URI.SpanURI()
	filename := uri.Filename()

	// update snapshot for changes seen
	err := d.ws.updateContent(ctx, uri, params.ContentChanges)
	if err != nil {
		d.log.Debug(err.Error())
		return nil
	}

	if err := d.ws.parseFile(filename); err != nil {
		d.log.Debug(err.Error())
		return nil
	}

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.ws.Validate(string(params.TextDocument.URI), d.ws.CorrespondingNodes)
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}

	return &lsp.PublishDiagnosticsParams{
		URI:         lsp.DocumentURI(params.TextDocument.URI),
		Diagnostics: diags,
	}
}

// DidOpen handles didOpen events.
func (d *Dispatcher) DidOpen(ctx context.Context, params lsp.DidOpenTextDocumentParams) *lsp.PublishDiagnosticsParams {
	if err := d.ws.Parse(); err != nil {
		// If we can't parse the workspace, log the error and skip validation.
		// TODO(hasheddan): surface this in diagnostics.
		d.log.Debug(errParseWorkspace, "error", err)
		return nil
	}
	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.ws.Validate(string(params.TextDocument.URI), d.ws.CorrespondingNodes)
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}
	return &lsp.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

// DidSave handles didSave events.
func (d *Dispatcher) DidSave(ctx context.Context, params lsp.DidSaveTextDocumentParams) *lsp.PublishDiagnosticsParams {
	if err := d.ws.Parse(); err != nil {
		// If we can't parse the workspace, log the error and skip validation.
		// TODO(hasheddan): surface this in diagnostics.
		d.log.Debug(errParseWorkspace, "error", err)
		return nil
	}

	// we saved the meta file, load validators if the file isn't invalid
	d.handleMeta(ctx, string(params.TextDocument.URI))

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.ws.Validate(string(params.TextDocument.URI), d.ws.CorrespondingNodes)
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}
	return &lsp.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

func (d *Dispatcher) handleMeta(_ context.Context, filename string) {
	if filepath.Base(filename) == xpkg.MetaFile {
		diags, err := d.ws.Validate(filename, d.ws.MetaNode)
		if err != nil {
			d.log.Debug(errValidateNodes, "error", err)
		}
		// don't load validators from the cache if the meta file is in an
		// invalid state.
		if len(diags) == 0 {
			// reset validators
			d.ws.ResetValidators()

			if err := d.ws.LoadValidators(d.root); err != nil {
				d.log.Debug(errLoadValidators, "error", err)
			}

			if err := d.ws.LoadCacheValidators(); err != nil {
				d.log.Debug(errLoadValidators, "error", err)
			}
			for k := range d.ws.snapshot.validators {
				d.log.Info(fmt.Sprintf("%+v", k))
			}
		}
	}
}
