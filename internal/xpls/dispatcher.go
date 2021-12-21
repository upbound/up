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
	"strings"
	"time"

	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"
	"github.com/radovskyb/watcher"
	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/xpkg"
)

var (
	// kind describes how text synchronization works.
	kind = lsp.TDSKIncremental
)

const (
	fileWatchGlob = "**/*.yaml"

	errParseWorkspace     = "failed to parse workspace"
	errValidateNodes      = "failed to validate nodes in workspace"
	errLoadValidators     = "failed to load validators"
	errPublishDiagnostics = "failed to publish diagnostics"

	errFailedToWatchCache = "failed to setup cache watch"
	errRegisteringWatches = "failed to register workspace watchers"
)

// Dispatcher --
type Dispatcher struct {
	clientConn *jsonrpc2.Conn

	root      span.URI
	cacheRoot string

	ws  *Workspace
	log logging.Logger

	watchInterval time.Duration
}

// NewDispatcher returns a new Dispatcher instance.
func NewDispatcher(log logging.Logger, cacheRoot string, watchInterval time.Duration) *Dispatcher {
	return &Dispatcher{
		cacheRoot: cacheRoot,
		log:       log,
		// TODO(@tnthornton) this shouldn't live here long term.
		watchInterval: watchInterval,
	}
}

// Initialize handles initialize events.
func (d *Dispatcher) Initialize(ctx context.Context, params protocol.InitializeParams, c *jsonrpc2.Conn) *lsp.InitializeResult {
	d.clientConn = c
	d.root = params.RootURI.SpanURI()
	ws, err := NewWorkspace(d.root, d.cacheRoot, WithWSLogger(d.log))
	if err != nil {
		panic(err)
	}

	d.ws = ws

	// TODO(@tnthornton) this is a slow operation
	if err := d.ws.LoadCacheValidators(); err != nil {
		// NOTE(@tnthornton) this error can happen if no dependencies for the
		// workspace currently exist in the cache.
		d.log.Debug(errLoadValidators, "error", err)
	}

	if err := d.ws.LoadValidators(d.root.Filename()); err != nil {
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
	diags, err := d.ws.Validate(lsp.DocumentURI(params.TextDocument.URI), d.ws.CorrespondingNodes)
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
	diags, err := d.ws.Validate(params.TextDocument.URI, d.ws.CorrespondingNodes)
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
	d.handleMeta(ctx, params.TextDocument.URI)

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.ws.Validate(params.TextDocument.URI, d.ws.CorrespondingNodes)
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}
	return &lsp.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

// WsWatchedFilesChanged handles didChangeWatchedFiles events.
func (d *Dispatcher) WsWatchedFilesChanged(ctx context.Context, params protocol.DidChangeWatchedFilesParams) []*lsp.PublishDiagnosticsParams {

	if err := d.ws.Parse(); err != nil {
		// If we can't parse the workspace, log the error and skip validation.
		// TODO(hasheddan): surface this in diagnostics.
		d.log.Debug(errParseWorkspace, "error", err)
		return nil
	}

	accDiags := make([]*lsp.PublishDiagnosticsParams, len(params.Changes))
	for _, c := range params.Changes {
		// only attempt to handle changes for files
		if strings.HasPrefix(c.URI.SpanURI().Filename(), fileProtocol) {
			// handle changes to meta file, if its a part of that slice
			d.handleMeta(ctx, lsp.DocumentURI(c.URI.SpanURI().Filename()))

			diags, err := d.ws.Validate(lsp.DocumentURI(c.URI), d.ws.CorrespondingNodes)
			if err != nil {
				d.log.Debug(errValidateNodes, "error", err)
				return nil
			}
			accDiags = append(accDiags, &lsp.PublishDiagnosticsParams{
				URI:         lsp.DocumentURI(c.URI),
				Diagnostics: diags,
			})
		}
	}

	return accDiags
}

func (d *Dispatcher) handleMeta(_ context.Context, filename lsp.DocumentURI) {
	if filepath.Base(string(filename)) == xpkg.MetaFile {
		diags, err := d.ws.Validate(filename, d.ws.MetaNode)
		if err != nil {
			d.log.Debug(errValidateNodes, "error", err)
		}
		// don't load validators from the cache if the meta file is in an
		// invalid state.
		if len(diags) == 0 {
			d.ws.ReloadValidators()
		}
	}
}

// watchCache watches the cache for changes.
// TODO(@tnthornton) this really doesn't feel like the correct place for this.
// I think we should eventually move this to the dep manager and subsequently notify
// the dispatcher that validations need to occur for the workspace.
// Unfortunately at this time, that will require a larger refactor.
func (d *Dispatcher) watchCache() { // nolint:gocyclo
	watch := watcher.New()

	watch.SetMaxEvents(1)

	d.log.Debug(fmt.Sprintf("cacheRoot: %s", d.ws.CacheRoot()))

	go func() {
		for {
			select {
			case event := <-watch.Event:
				go func() {
					d.log.Debug(fmt.Sprintf("event: %s", event))
					if err := watch.AddRecursive(d.ws.CacheRoot()); err != nil {
						d.log.Debug(errFailedToWatchCache, "error", err)
					}
					for path, f := range watch.WatchedFiles() {
						d.log.Debug(fmt.Sprintf("%s: %s\n", path, f.Name()))
					}

					d.ws.ReloadValidators()

					params := make([]*lsp.PublishDiagnosticsParams, 0)
					for _, n := range d.ws.AllNodes("") {
						gvk := n.GetGVK()
						v, ok := d.ws.snapshot.validators[gvk]
						if !ok {
							continue
						}
						params = append(params, &lsp.PublishDiagnosticsParams{
							URI:         lsp.DocumentURI(fmt.Sprintf(fileProtocolFmt, n.GetFileName())),
							Diagnostics: validationDiagnostics(v.Validate(n.GetObject()), n.GetAST(), n.GetGVK()),
						})
					}

					// TODO(@tnthornton) do we really need to iterate through
					// this separately from the above loop?
					for _, p := range params {
						d.publishDiagnostics(context.Background(), p)
					}
				}()

			case err := <-watch.Error:
				d.log.Debug(err.Error())
			case <-watch.Closed:
				return
			}
		}
	}()

	// Watch cache root directory recursively for changes.
	if err := watch.AddRecursive(d.ws.CacheRoot()); err != nil {
		d.log.Debug(errFailedToWatchCache, "error", err)
	}

	// Print a list of all of the files and folders currently
	// being watched and their paths.
	for path, f := range watch.WatchedFiles() {
		d.log.Debug(fmt.Sprintf("%s: %s\n", path, f.Name()))
	}

	// Start the watching process - it'll check for changes every 100ms.
	go func() {
		if err := watch.Start(d.watchInterval); err != nil {
			d.log.Debug(errFailedToWatchCache, "error", err)
		}
	}()
}

func (d *Dispatcher) publishDiagnostics(ctx context.Context, params *lsp.PublishDiagnosticsParams) {
	if err := d.clientConn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
		d.log.Debug(errPublishDiagnostics, "error", err)
	}
}

func (d *Dispatcher) registerWatchFilesCapability() {
	go func() {
		if err := d.clientConn.Call(context.Background(), "client/registerCapability", &protocol.RegistrationParams{
			Registrations: []protocol.Registration{
				{
					ID:     "workspace/didChangeWatchedFiles-1",
					Method: "workspace/didChangeWatchedFiles",
					RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
						Watchers: []protocol.FileSystemWatcher{
							{
								GlobPattern: fileWatchGlob,
								Kind:        uint32(protocol.WatchChange + protocol.WatchDelete + protocol.WatchCreate),
							},
						},
					},
				},
			},
		}, nil); err != nil {
			d.log.Debug(errRegisteringWatches, "error", err)
		}
	}()
}
