// Copyright 2023 Upbound Inc
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

package controlplane

import (
	"time"
)

// Response is a normalized ControlPlane response.
// NOTE(tnthornton) this is expected to be different in the near future as
// cloud and spaces APIs converge.
type Response struct {
	ID                string
	Group             string
	Name              string
	CrossplaneVersion string
	Ready             string
	Message           string
	Age               *time.Duration

	Cfg     string
	Updated string

	ConnName string
}
