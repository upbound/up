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
	"strings"
	"sync"
	"time"

	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"
	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/snapshot"
)

var (
	// kind describes how text synchronization works.
	kind = lsp.TDSKIncremental
)

const (
	fileProtocol  = "file://"
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
	mu sync.RWMutex

	clientConn *jsonrpc2.Conn

	root      span.URI
	cacheRoot string

	snapFactory *snapshot.Factory
	snap        *snapshot.Snapshot
	log         logging.Logger

	watchInterval *time.Duration
}

// NewDispatcher returns a new Dispatcher instance.
func NewDispatcher(log logging.Logger, cacheRoot string, watchInterval *time.Duration) *Dispatcher {
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

	m, err := manager.New(
		manager.WithLogger(d.log),
		manager.WithWatchInterval(d.watchInterval),
	)
	if err != nil {
		panic(err)
	}

	factory, err := snapshot.NewFactory(
		d.root.Filename(),
		snapshot.WithLogger(d.log),
		snapshot.WithDepManager(m),
	)
	if err != nil {
		panic(err)
	}

	d.snapFactory = factory

	snap, err := d.snapFactory.New()
	if err != nil {
		panic(err)
	}

	d.snap = snap

	// TODO(@tnthornton) this is a slow operation
	// if err := d.ws.LoadCacheValidators(); err != nil {
	// 	// NOTE(@tnthornton) this error can happen if no dependencies for the
	// 	// workspace currently exist in the cache.
	// 	d.log.Debug(errLoadValidators, "error", err)
	// }

	// if err := d.ws.LoadValidators(d.root.Filename()); err != nil {
	// 	// If we can't load validators panic because we won't be able to
	// 	// perform validation.
	// 	panic(err)
	// }

	// if err := d.ws.Parse(); err != nil {
	// 	d.log.Debug(errParseWorkspace, "error", err)
	// 	panic(err)
	// }

	d.watchSnapshot()

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Kind: &kind,
			},
		},
	}
}

// DidChange handles didChange events.
func (d *Dispatcher) DidChange(ctx context.Context, params protocol.DidChangeTextDocumentParams) *protocol.PublishDiagnosticsParams {
	uri := params.TextDocument.URI.SpanURI()
	filename := uri.Filename()

	// update snapshot for changes seen
	err := d.snap.UpdateContent(ctx, uri, params.ContentChanges)
	if err != nil {
		d.log.Debug(err.Error())
		return nil
	}

	if err := d.snap.ReParseFile(filename); err != nil {
		d.log.Debug(err.Error())
		return nil
	}

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.snap.Validate(params.TextDocument.URI.SpanURI())
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}

	return &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

// DidOpen handles didOpen events.
func (d *Dispatcher) DidOpen(ctx context.Context, params protocol.DidOpenTextDocumentParams) *protocol.PublishDiagnosticsParams {
	// if err := d.ws.Parse(); err != nil {
	// 	// If we can't parse the workspace, log the error and skip validation.
	// 	// TODO(hasheddan): surface this in diagnostics.
	// 	d.log.Debug(errParseWorkspace, "error", err)
	// 	return nil
	// }
	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.snap.Validate(params.TextDocument.URI.SpanURI())
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}
	return &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

// DidSave handles didSave events.
func (d *Dispatcher) DidSave(ctx context.Context, params protocol.DidSaveTextDocumentParams) *protocol.PublishDiagnosticsParams {
	d.mu.Lock()
	defer d.mu.Unlock()
	// create new snapshot here
	snap, err := d.snapFactory.New()
	if err != nil {
		d.log.Debug(errParseWorkspace, "error", err)
		return nil
	}
	// TODO(@tnthornton) this isn't thread safe until we serialize incoming requests.
	d.snap = snap

	// if err := d.ws.Parse(); err != nil {
	// 	// If we can't parse the workspace, log the error and skip validation.
	// 	// TODO(hasheddan): surface this in diagnostics.
	// 	d.log.Debug(errParseWorkspace, "error", err)
	// 	return nil
	// }

	// we saved the meta file, load validators if the file isn't invalid
	// d.handleMeta(ctx, params.TextDocument.URI)

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := d.snap.Validate(params.TextDocument.URI.SpanURI())
	if err != nil {
		d.log.Debug(errValidateNodes, "error", err)
		return nil
	}
	return &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
}

// WsWatchedFilesChanged handles didChangeWatchedFiles events.
func (d *Dispatcher) WsWatchedFilesChanged(ctx context.Context, params protocol.DidChangeWatchedFilesParams) []*protocol.PublishDiagnosticsParams {
	d.mu.Lock()
	defer d.mu.Unlock()

	snap, err := d.snapFactory.New()
	if err != nil {
		d.log.Debug(errParseWorkspace, "error", err)
		return nil
	}
	// TODO(@tnthornton) this isn't thread safe until we serialize incoming requests.
	d.snap = snap
	// if err := d.ws.Parse(); err != nil {
	// 	// If we can't parse the workspace, log the error and skip validation.
	// 	// TODO(hasheddan): surface this in diagnostics.
	// 	d.log.Debug(errParseWorkspace, "error", err)
	// 	return nil
	// }

	accDiags := make([]*protocol.PublishDiagnosticsParams, len(params.Changes))
	for _, c := range params.Changes {
		// only attempt to handle changes for files
		if strings.HasPrefix(c.URI.SpanURI().Filename(), fileProtocol) {
			// handle changes to meta file, if its a part of that slice
			// d.handleMeta(ctx, lsp.DocumentURI(c.URI.SpanURI().Filename()))

			diags, err := d.snap.Validate(c.URI.SpanURI())
			if err != nil {
				d.log.Debug(errValidateNodes, "error", err)
				return nil
			}
			accDiags = append(accDiags, &protocol.PublishDiagnosticsParams{
				URI:         c.URI,
				Diagnostics: diags,
			})
		}
	}

	return accDiags
}

// func (d *Dispatcher) handleMeta(_ context.Context, filename lsp.DocumentURI) {
// 	if filepath.Base(string(filename)) == xpkg.MetaFile {
// 		diags, err := d.ws.Validate(filename, d.ws.MetaNode)
// 		if err != nil {
// 			d.log.Debug(errValidateNodes, "error", err)
// 		}
// 		// don't load validators from the cache if the meta file is in an
// 		// invalid state.
// 		if len(diags) == 0 {
// 			d.ws.ReloadValidators()
// 		}
// 	}
// }

// // watchSnapshot watches the cache for changes.
func (d *Dispatcher) watchSnapshot() { // nolint:gocyclo
	watch := d.snapFactory.WatchExt()

	go func() {
		for {
			// TODO(@tnthornton) handle error/close case from cache
			<-watch
			d.log.Debug("change seen at cache, processing...")
			go func() {
				d.mu.Lock()
				defer d.mu.Unlock()
				snap, err := d.snapFactory.New()
				if err != nil {
					d.log.Debug(err.Error())
				}
				d.snap = snap

				params := make([]*protocol.PublishDiagnosticsParams, 0)

				validations, err := snap.ValidateAllFiles()
				if err != nil {
					d.log.Debug(err.Error())
					return
				}

				for uri, diags := range validations {
					params = append(params, &protocol.PublishDiagnosticsParams{
						URI:         protocol.URIFromSpanURI(uri),
						Diagnostics: diags,
					})
				}

				// TODO(@tnthornton) do we really need to iterate through
				// this separately from the above loop?
				for _, p := range params {
					d.publishDiagnostics(context.Background(), p)
				}
			}()
		}
	}()
}

func (d *Dispatcher) publishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) {
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
