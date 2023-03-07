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

package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"
	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/version"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/snapshot"
)

var (
	// kind describes how text synchronization works.
	kind = lsp.TDSKIncremental
)

const (
	defaultWatchInterval = "100ms"
	fileProtocol         = "file://"
	fileWatchGlob        = "**/*.yaml"
	newVersionMsgFmt     = `Version %s of up is now available. Current version is %s.
	Update for the latest features!`

	errParseWorkspace     = "failed to parse workspace"
	errPublishDiagnostics = "failed to publish diagnostics"
	errRegisteringWatches = "failed to register workspace watchers"
	errValidateMeta       = "failed to validate crossplane.yaml file in workspace"
	errShowMessage        = "failed to show message"
	errValidateNodes      = "failed to validate nodes in workspace"
)

// Server services incoming LSP requests.
type Server struct {
	conn *jsonrpc2.Conn

	i   *version.Informer
	log logging.Logger
	m   *manager.Manager
	mu  sync.RWMutex

	root span.URI

	snapFactory *snapshot.Factory
	snap        *snapshot.Snapshot
}

// New returns a new Server.
func New(opts ...Option) (*Server, error) {
	s := &Server{
		log: logging.NewNopLogger(),
	}

	interval, err := time.ParseDuration(defaultWatchInterval)
	if err != nil {
		return nil, err
	}

	// TODO(@tnthornton) supply cache root from Config here.
	m, err := manager.New(
		manager.WithLogger(s.log),
		manager.WithWatchInterval(&interval),
	)
	if err != nil {
		return nil, err
	}

	s.m = m

	s.i = version.NewInformer(version.WithLogger(s.log))

	return s, nil
}

// Option provides a way to override default behavior of the Server.
type Option func(*Server)

// WithLogger overrides the default logging.Logger for the Server with the
// supplied logging.Logger.
func WithLogger(l logging.Logger) Option {
	return func(s *Server) {
		s.log = l
	}
}

// Initialize handles calls to Initialize.
func (s *Server) Initialize(ctx context.Context, conn *jsonrpc2.Conn, id jsonrpc2.ID, params *protocol.InitializeParams) {

	// TODO(@tnthornton) this is the only place that the passed in conn is used.
	// Given that the conn is the same at the time it is established, we should
	// work towards pulling out this dependency into something we can supply.
	// It will make testing easier if we are in control of it versus relying on
	// the handler to pass it down.
	s.conn = conn
	s.root = params.RootURI.SpanURI()

	factory, err := snapshot.NewFactory(
		s.root.Filename(),
		snapshot.WithLogger(s.log),
		snapshot.WithDepManager(s.m),
	)
	if err != nil {
		panic(err)
	}

	s.snapFactory = factory

	snap, err := s.snapFactory.New(ctx)
	if err != nil {
		panic(err)
	}

	s.snap = snap

	s.watchSnapshot(context.Background()) //nolint:contextcheck  // TODO(epk) thread through top level context

	// TODO (@tnthornton) move to using protocol.InitializeResult
	reply := &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Kind: &kind,
			},
		},
	}

	if err := s.conn.Reply(ctx, id, reply); err != nil {
		// If we fail to initialize the workspace we won't receive future
		// messages so we panic and try again on restart.
		panic(err)
	}

	s.registerWatchFilesCapability(context.Background()) //nolint:contextcheck // TODO(epk) thread through top level context
	s.checkMetaFile(context.Background())                //nolint:contextcheck // TODO(epk) thread through top level context
	s.checkForUpdates(context.Background())              //nolint:contextcheck // TODO(epk) thread through top level context
}

// DidChange handles calls to DidChange.
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) {
	uri := params.TextDocument.URI.SpanURI()
	filename := uri.Filename()

	// update snapshot for changes seen
	err := s.snap.UpdateContent(ctx, uri, params.ContentChanges)
	if err != nil {
		s.log.Debug(err.Error())
		return
	}

	if err := s.snap.ReParseFile(ctx, filename); err != nil {
		s.log.Debug(err.Error())
		return
	}

	// TODO(hasheddan): diagnostics should be cached and validation should
	// be performed selectively.
	diags, err := s.snap.Validate(ctx, params.TextDocument.URI.SpanURI())
	if err != nil {
		s.log.Debug(errValidateNodes, "error", err)
		return
	}
	reply := &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
	s.publishDiagnostics(ctx, reply)
}

// DidOpen handles calls to DidOpen.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) {
	diags, err := s.snap.Validate(ctx, params.TextDocument.URI.SpanURI())
	if err != nil {
		s.log.Debug(errValidateNodes, "error", err)
		return
	}
	reply := &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
	s.publishDiagnostics(ctx, reply)
}

// DidSave handles calls to DidSave.
func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// create new snapshot here
	snap, err := s.snapFactory.New(ctx)
	if err != nil {
		s.log.Debug(errParseWorkspace, "error", err)
		return
	}
	s.snap = snap

	diags, err := s.snap.Validate(ctx, params.TextDocument.URI.SpanURI())
	if err != nil {
		s.log.Debug(errValidateNodes, "error", err)
		return
	}
	reply := &protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: diags,
	}
	s.publishDiagnostics(ctx, reply)
}

// DidChangeWatchedFiles handles calls to DidChangeWatchedFiles.
func (s *Server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, err := s.snapFactory.New(ctx)
	if err != nil {
		s.log.Debug(errParseWorkspace, "error", err)
		return
	}
	s.snap = snap

	accDiags := make([]*protocol.PublishDiagnosticsParams, 0)
	for _, c := range params.Changes {
		// only attempt to handle changes for files
		if strings.HasPrefix(c.URI.SpanURI().Filename(), fileProtocol) {
			diags, err := s.snap.Validate(ctx, c.URI.SpanURI())
			if err != nil {
				s.log.Debug(errValidateNodes, "error", err)
				return
			}
			accDiags = append(accDiags, &protocol.PublishDiagnosticsParams{
				URI:         c.URI,
				Diagnostics: diags,
			})
		}
	}

	// iterate over the list of diagnostics and return individual reports
	for _, d := range accDiags {
		s.publishDiagnostics(ctx, d)
	}
}

func (s *Server) publishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) {
	if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
		s.log.Debug(errPublishDiagnostics, "error", err)
	}
}

func (s *Server) showMessage(ctx context.Context, params *protocol.ShowMessageParams) {
	if err := s.conn.Notify(ctx, "window/showMessage", params); err != nil {
		s.log.Debug(errShowMessage, "error", err)
	}
}

func (s *Server) registerWatchFilesCapability(ctx context.Context) {
	go func() {
		if err := s.conn.Call(ctx, "client/registerCapability", &protocol.RegistrationParams{
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
			s.log.Debug(errRegisteringWatches, "error", err)
		}
	}()
}

func (s *Server) checkForUpdates(ctx context.Context) {
	go func() {
		local, remote, ok := s.i.CanUpgrade(ctx)
		if !ok {
			// can't upgrade, nothing to do
			return
		}

		s.showMessage(ctx, &protocol.ShowMessageParams{
			Type:    protocol.Info,
			Message: fmt.Sprintf(newVersionMsgFmt, remote, local),
		})
	}()
}

func (s *Server) checkMetaFile(ctx context.Context) {
	go func() {
		uri, diags, err := s.snap.ValidateMeta(ctx)
		if err != nil {
			s.log.Debug(errValidateMeta, "error", err)
			return
		}
		s.publishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         protocol.URIFromSpanURI(uri),
			Diagnostics: diags,
		})
	}()
}

// // watchSnapshot watches the cache for changes.
func (s *Server) watchSnapshot(ctx context.Context) { // nolint:gocyclo
	watch := s.snapFactory.WatchExt()

	go func() {
		for {
			// TODO(@tnthornton) handle error/close case from cache
			<-watch
			s.log.Debug("change seen at cache, processing...")
			go func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				snap, err := s.snapFactory.New(ctx)
				if err != nil {
					s.log.Debug(err.Error())
				}
				s.snap = snap

				params := make([]*protocol.PublishDiagnosticsParams, 0)

				validations, err := snap.ValidateAllFiles(ctx)
				if err != nil {
					s.log.Debug(err.Error())
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
					s.publishDiagnostics(ctx, p)
				}
			}()
		}
	}()
}
