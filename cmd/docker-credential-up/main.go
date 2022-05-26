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
	"os"

	"github.com/docker/docker-credential-helpers/credentials"

	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/version"
)

const (
	profileEnv = "UP_PROFILE"
	domainEnv  = "UP_DOMAIN"
)

func main() {
	var v bool
	flag.BoolVar(&v, "v", false, "Print CLI version and exit.")
	flag.Parse()

	if v {
		fmt.Fprintln(os.Stdout, version.GetVersion())
		os.Exit(0)
	}

	// Build credential helper and defer execution to Docker.
	h := credhelper.New(
		credhelper.WithDomain(os.Getenv(domainEnv)),
		credhelper.WithProfile(os.Getenv(profileEnv)),
	)
	credentials.Serve(h)
}
