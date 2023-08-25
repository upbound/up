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

package model

import (
	"time"
)

// MXPGVKEvent records an event associated with an MXP and k8s GVK.
type MXPGVKEvent struct {
	Name         string          `json:"name"`
	Tags         MXPGVKEventTags `json:"tags"`
	Timestamp    time.Time       `json:"timestamp"`
	TimestampEnd time.Time       `json:"timestamp_end"`
	Value        float64         `json:"value"`
}

type MXPGVKEventTags struct {
	Group          string `json:"customresource_group"`
	Version        string `json:"customresource_version"`
	Kind           string `json:"customresource_kind"`
	UpboundAccount string `json:"upbound_account"`
	MXPID          string `json:"mxp_id"`
}
