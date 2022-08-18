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
	"os/signal"
	"syscall"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/sourcegraph/jsonrpc2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/upbound/up/internal/xpls"
	"github.com/upbound/up/internal/xpls/handler"
)

// serveCmd starts the language server.
type serveCmd struct {
	// TODO(@tnthornton) cache dir doesn't seem to be the responsibility of the
	// serve command. It seems like we can easily get into an inconsistent state
	// if someone specifies config element from the command line. We should move
	// this to the config.
	Cache   string `default:"~/.up/cache" help:"Directory path for dependency schema cache." type:"path"`
	Verbose bool   `help:"Run server with verbose logging."`
}

// Run runs the language server.
func (c *serveCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// cache directory resolution should occur at this level.

	// TODO(hasheddan): move to AfterApply.
	zl := zap.New(zap.UseDevMode(c.Verbose))
	h, err := handler.New(
		handler.WithLogger(logging.NewLogrLogger(zl.WithName("xpls"))),
	)
	if err != nil {
		return err
	}

	conn := jsonrpc2.NewConn(ctx, jsonrpc2.NewBufferedStream(xpls.StdRWC{}, jsonrpc2.VSCodeObjectCodec{}), h)
	go func() {
		for {
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-conn.DisconnectNotify():
				stop()
			}
		}
	}()

	<-ctx.Done()
	return nil
}
