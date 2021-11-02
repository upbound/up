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

package templates

import (
	"bytes"
	"errors"
	"text/template"

	"github.com/upbound/up/internal/xpkg"
)

const (
	errXPkgNameNotProvided  = "package name not provided"
	errCtrlImageNotProvided = "controller images not provided"

	// defines the configuration template
	configTmpl = `apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: {{ .Name }}
{{- if or .DependsOn .XPVersion }}
spec:
  {{- if .XPVersion }}
  crossplane:
    version: {{ .XPVersion }}
  {{- end -}}
  {{- if .DependsOn }}
  dependsOn:
    {{- range .DependsOn }}
    - provider: {{ .Provider }}
      version: {{ .Version -}}
    {{ end -}}
  {{ end -}}
{{ end -}}`

	// defines the provider template
	provTmpl = `apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: {{ .Name }}
spec:
  {{- if .XPVersion }}
  crossplane:
    version: {{ .XPVersion }}
  {{- end }}
  controller:
    image: {{ .CtrlImage }}`
)

// NewConfigXPkg returns a slice of bytes containing a fully rendered
// Configuration template given the provided ConfigContext.
func NewConfigXPkg(c xpkg.InitContext) ([]byte, error) {
	// name is required
	if c.Name == "" {
		return nil, errors.New(errXPkgNameNotProvided)
	}

	return parseXPkg(c, configTmpl)
}

// NewProviderXPkg returns a slice of bytes containing a fully rendered
// Provider template given the provided ProviderContext.
func NewProviderXPkg(c xpkg.InitContext) ([]byte, error) {
	// name is required
	if c.Name == "" {
		return nil, errors.New(errXPkgNameNotProvided)
	}

	// image is required
	if c.CtrlImage == "" {
		return nil, errors.New(errCtrlImageNotProvided)
	}

	return parseXPkg(c, provTmpl)
}

func parseXPkg(ctx interface{}, tmpl string) ([]byte, error) {
	var buf bytes.Buffer

	t := template.New("xpkg")

	t, err := t.Parse(tmpl)
	if err != nil {
		return nil, err
	}

	if err := t.Execute(&buf, ctx); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
