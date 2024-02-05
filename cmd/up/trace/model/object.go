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

package model

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	queryv1alpha1 "github.com/upbound/up/cmd/up/trace/query/v1alpha1"
)

type Object struct {
	Id string

	Group, Kind     string
	ControlPlane    ControlPlane
	Namespace, Name string

	DeletionTimestamp, CreationTimestamp time.Time

	Synced []Interval
	Ready  []Interval
	JSON   queryv1alpha1.JSONObject
	Events []Event

	Children []*Object
}

type Interval struct {
	From, To time.Time
}

type ControlPlane struct {
	Namespace, Name string
}

type Event struct {
	LastTimestamp metav1.Time `json:"lastTimestamp"`
	Message       string      `json:"message"`
	Count         int         `json:"count"`
	Type          string      `json:"type"`
}

func (o *Object) Title() string {
	prefix := ""
	if !o.DeletionTimestamp.IsZero() {
		prefix = "[:#ff0000]💀[:-] "
	}
	if o.Namespace == "" {
		return fmt.Sprintf("%s%s [darkgrey]%s[-]", prefix, o.Kind, o.Name)
	}
	return fmt.Sprintf("%s%s [darkgrey]%s[-][::b]/[::-][darkgrey]%s[-]", prefix, o.Kind, o.Namespace, o.Name)
}

func (o *Object) IsSynced(ts time.Time) bool {
	for i := range o.Synced {
		if o.Synced[i].From.After(ts) {
			return false
		}
		if o.Synced[i].To.IsZero() || o.Synced[i].To.After(ts) {
			return true
		}
	}
	return false
}

func (o *Object) IsReady(ts time.Time) bool {
	for i := range o.Ready {
		if o.Ready[i].From.After(ts) {
			return false
		}
		if o.Ready[i].To.IsZero() || o.Ready[i].To.After(ts) {
			return true
		}
	}
	return false
}
