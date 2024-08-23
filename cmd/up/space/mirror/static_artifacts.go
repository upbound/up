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
	"encoding/json"
	"reflect"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
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

type KubeVersionPath struct {
	ControlPlanes struct {
		K8sVersion StringOrArray `json:"k8sVersion"`
	} `json:"controlPlanes"`
}

func (j *UXPVersionsPath) GetSupportedVersions() ([]string, error) {
	if len(j.Controller.Crossplane.SupportedVersions) == 0 {
		return nil, errors.New("no supported versions found in UXPVersionsPath")
	}
	return j.Controller.Crossplane.SupportedVersions, nil
}

func (k *KubeVersionPath) GetSupportedVersions() ([]string, error) {
	if len(k.ControlPlanes.K8sVersion) == 0 {
		return nil, errors.New("no supported versions found in KubeVersionPath")
	}
	return k.ControlPlanes.K8sVersion, nil
}

// init function to return byte slice and oci.PathNavigator
func initConfig() ([]byte, map[string]reflect.Type) {
	return configFile, map[string]reflect.Type{
		"uxpVersionsPath": reflect.TypeOf(UXPVersionsPath{}),
		"kubeVersionPath": reflect.TypeOf(KubeVersionPath{}),
	}
}

type StringOrArray []string

func (s *StringOrArray) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}

	var array []string
	if err := json.Unmarshal(data, &array); err == nil {
		*s = array
		return nil
	}

	return errors.New("data is neither a string nor an array of strings")
}
