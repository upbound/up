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
	"encoding/json"
	"io"
	"strings"

	"github.com/alecthomas/kong"
)

// JSON returns a Resolver that retrieves values from a JSON source.
// Based slightly off of https://github.com/alecthomas/kong/blob/f48da244f54370c0cb63e22b0e500e5459a491bf/resolver.go#L33-L60
// Hyphens in flag names are replaced with underscores.
func JSON(base, overlay io.Reader) (kong.Resolver, error) {
	baseValues := map[string]interface{}{}
	overlayValues := map[string]interface{}{}
	err := json.NewDecoder(base).Decode(&baseValues)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(overlay).Decode(&overlayValues)
	if err != nil {
		return nil, err
	}

	var f kong.ResolverFunc = func(context *kong.Context, parent *kong.Path, flag *kong.Flag) (interface{}, error) {
		name := strings.ReplaceAll(flag.Name, "-", "_")
		bRaw, bOk := resolveValue(name, flag.Envs, baseValues)
		oRaw, oOk := resolveValue(name, flag.Envs, overlayValues)

		// if found in base and in overlay AND is not the defaultValue for overlay
		if bOk && oOk && stringify(oRaw) != flag.Default {
			return oRaw, nil
		}

		if bOk {
			return stringify(bRaw), nil
		}

		if oOk {
			return stringify(oRaw), nil
		}

		return nil, nil
	}

	return f, nil
}

func resolveValue(fieldName string, envVarNames []string, vals map[string]interface{}) (interface{}, bool) {
	// attempt to lookup by field name first
	raw, ok := vals[fieldName]
	if !ok {
		// fall back to env var name
		for _, envVarName := range envVarNames {
			raw, ok = vals[envVarName]
			if ok {
				return raw, true
			}
		}
		return nil, false
	}
	return raw, true
}

func stringify(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	bs, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bs)
}
