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

package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"

	"github.com/docker/docker-credential-helpers/credentials"

	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/version"
)

const (
	profileEnv = "UP_PROFILE"
	domainEnv  = "UP_DOMAIN"
)

const (
	errInvalidDomain = "invalid value for UP_DOMAIN"
)

func main() {
	var v bool
	flag.BoolVar(&v, "v", false, "Print CLI version and exit.")
	flag.Parse()

	if v {
		fmt.Fprintln(os.Stdout, version.Version())
		os.Exit(0)
	}

	domain := ""
	if de, ok := os.LookupEnv(domainEnv); ok {
		u, err := url.Parse(de)
		if err != nil {
			fmt.Fprintln(os.Stdout, errInvalidDomain)
			os.Exit(1)
		}
		domain = u.Hostname()
	}

	// Build credential helper and defer execution to Docker.
	h := credhelper.New(
		credhelper.WithDomain(domain),
		credhelper.WithProfile(os.Getenv(profileEnv)),
	)
	credentials.Serve(h)
}
