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
package upterm

import (
	"encoding/json"
	"fmt"

	"github.com/upbound/up/cmd/up/globals"

	"gopkg.in/yaml.v3"
)

// Print an objet in either JSON or YAML
func PrintFormatted(format string, obj any) error {
	if format == globals.OutputJSON {
		return printJSON(obj)
	}
	if format == globals.OutputYAML {
		return printYAML(obj)
	}
	return fmt.Errorf("Unknown format: %s", format)
}

func printJSON(obj any) error {
	js, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(js))
	return err
}

func printYAML(obj any) error {
	ys, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(ys))
	return err
}
