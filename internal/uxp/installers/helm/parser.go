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

package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/strvals"

	"github.com/upbound/up/internal/uxp"
)

// Parser is a helm-style parameter parser.
type Parser struct {
	values    map[string]interface{}
	overrides map[string]string
}

// NewParser returns a parameter parser backed by helm.
func NewParser(base map[string]interface{}, overrides map[string]string) uxp.ParameterParser {
	return &Parser{
		values:    base,
		overrides: overrides,
	}
}

// Parse parses install and upgrade parameters
func (p *Parser) Parse() (map[string]interface{}, error) {
	for k, v := range p.overrides {
		if err := strvals.ParseInto(fmt.Sprintf("%s=%s", k, v), p.values); err != nil {
			return nil, err
		}
	}
	return p.values, nil
}
