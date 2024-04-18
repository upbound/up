// Copyright 2024 Upbound Inc
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
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/upbound/up-sdk-go/service/accounts"
)

func GetAccount(ctx context.Context, ac *accounts.Client, account string) (*accounts.AccountResponse, error) {
	a, err := ac.Get(ctx, account)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Account %q", account)
	}
	if a.Account.Type != accounts.AccountOrganization {
		return nil, fmt.Errorf("account %q is not an organization", account)
	}
	if a.Organization == nil {
		return nil, fmt.Errorf("account %q does not have an organization", account)
	}
	return a, nil
}
