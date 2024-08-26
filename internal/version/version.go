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

package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	productName = "up-cli"

	// 5 seconds should be more than enough time.
	clientTimeout = 5 * time.Second
	cliURL        = "https://cli.upbound.io/stable/current/version"

	errFailedToQueryRemoteFmt = "query to %s failed"
	errInvalidLocalVersion    = "invalid local version detected"
	errInvalidRemoteVersion   = "invalid remote version detected"
	errNotSemVerFmt           = "%s; couldn't covert version to semver"
)

type releaseTarget string

const (
	ReleaseTargetRelease releaseTarget = "release"
	ReleaseTargetDebug   releaseTarget = "debug"
)

var (
	version             string
	agentVersion        string = "0.0.0-416.g084be68"
	mcpConnectorVersion string = "0.7.0"
	gitCommit           string = "unknown-commit"
	target              string = string(ReleaseTargetDebug)
)

func UserAgent() string {
	return fmt.Sprintf("%s/%s (%s; %s)", productName, version, runtime.GOOS, runtime.GOARCH)
}

// Version returns the current build version.
func Version() string {
	return version
}

// GitCommit returns the commit SHA that was used to build the current version.
func GitCommit() string {
	return gitCommit
}

// AgentVersion returns the connect agent version.
func AgentVersion() string {
	return agentVersion
}

// MCPConnectorVersion returns the connector version.
func MCPConnectorVersion() string {
	return mcpConnectorVersion
}

// ReleaseTarget returns the target type that the binary was built with.
func ReleaseTarget() releaseTarget {
	switch target {
	case string(ReleaseTargetRelease):
		return ReleaseTargetRelease
	case string(ReleaseTargetDebug):
		fallthrough
	default:
		return ReleaseTargetDebug
	}
}

type client interface {
	Do(*http.Request) (*http.Response, error)
}

type defaultClient struct {
	client http.Client
}

// Informer enables the caller to determine if they can upgrade their current
// version of up.
type Informer struct {
	client client
	log    logging.Logger
}

// NewInformer constructs a new Informer.
func NewInformer(opts ...Option) *Informer {
	i := &Informer{
		log:    logging.NewNopLogger(),
		client: newClient(),
	}

	for _, o := range opts {
		o(i)
	}

	return i
}

// Option modifies the Informer.
type Option func(*Informer)

// WithLogger overrides the default logger for the Informer.
func WithLogger(l logging.Logger) Option {
	return func(i *Informer) {
		i.log = l
	}
}

// CanUpgrade queries locally for the version of up, uses the Informer's client
// to check what the currently published version of up is and returns the local
// and remote versions and whether or not we could upgrade up.
func (i *Informer) CanUpgrade(ctx context.Context) (string, string, bool) {
	local := Version()
	remote, err := i.getCurrent(ctx)
	if err != nil {
		i.log.Debug(fmt.Sprintf(errFailedToQueryRemoteFmt, cliURL), "error", err)
		return "", "", false
	}

	return local, remote, i.newAvailable(local, remote)
}

func (i *Informer) newAvailable(local, remote string) bool {
	lv, err := semver.NewVersion(local)
	if err != nil {
		//
		i.log.Debug(fmt.Sprintf(errNotSemVerFmt, errInvalidLocalVersion), "error", err)
		return false
	}
	rv, err := semver.NewVersion(remote)
	if err != nil {
		// invalid remote version detected
		i.log.Debug(fmt.Sprintf(errNotSemVerFmt, errInvalidRemoteVersion), "error", err)
		return false
	}

	return rv.GreaterThan(lv)
}

func (i *Informer) getCurrent(ctx context.Context) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, cliURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := i.client.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() // nolint:gosec,errcheck

	v, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(v), "\n"), nil
}

func newClient() *defaultClient {
	return &defaultClient{
		client: http.Client{
			Timeout: clientTimeout,
		},
	}
}

func (d *defaultClient) Do(r *http.Request) (*http.Response, error) {
	return d.client.Do(r)
}
