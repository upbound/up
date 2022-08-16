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

package upbound

import (
	"fmt"

	"github.com/upbound/up/internal/config"
)

var (
	selfhosted = "selfhosted"
	account    = "my-org"

	// TODO(tnthornton) see if we can use Kong interpolation to define these
	// environment variable names once and reuse them in the various commands.
	accountKey            = "UP_ACCOUNT"
	domainKey             = "UP_DOMAIN"
	insecureTLSSkipVerify = "UP_INSECURE_SKIP_TLS_VERIFY"
	mcpExperimentalKey    = "UP_MCP_EXPERIMENTAL"
)

// getSelfHostedProfile returns the standard profile we set up for selfhosted
// installs. This includes the profile name (first returned value) as well as
// the base config itself.
func getSelfHostedProfile(domain string) (string, config.Profile) {
	return selfhosted, config.Profile{
		ID:      "to-be-updated",
		Account: account,
		Type:    config.UserProfileType,
		BaseConfig: map[string]string{
			accountKey:            account,
			domainKey:             fmt.Sprintf("https://%s", domain),
			insecureTLSSkipVerify: "true",
			mcpExperimentalKey:    "true",
		},
	}
}
