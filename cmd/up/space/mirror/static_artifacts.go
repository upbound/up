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

package mirror

import (
	_ "embed"
	"reflect"
)

// Embed the YAML file.
//
//go:embed config.yaml
var configFile []byte

type UXPVersionsPath struct {
	Controller struct {
		Crossplane struct {
			SupportedVersions []string `json:"supportedVersions"`
		} `json:"crossplane"`
	} `json:"controller"`
}

func (j *UXPVersionsPath) GetSupportedVersions() ([]string, error) {
	return j.Controller.Crossplane.SupportedVersions, nil
}

// init function to return byte slice and oci.PathNavigator
func initConfig() ([]byte, map[string]reflect.Type) {
	return configFile, map[string]reflect.Type{"uxpVersionsPath": reflect.TypeOf(UXPVersionsPath{})}
}
