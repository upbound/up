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
	"github.com/rivo/tview"
)

type ObjectsOrder []*tview.TreeNode

func (o ObjectsOrder) Len() int { return len(o) }
func (o ObjectsOrder) Less(i, j int) bool {
	oi := o[i].GetReference().(*Object)
	oj := o[i].GetReference().(*Object)

	if a, b := oi.Group, oj.Group; a != b {
		return a < b
	}
	if a, b := oi.Kind, oj.Kind; a != b {
		return a < b
	}
	if a, b := oi.ControlPlane.Namespace, oj.ControlPlane.Namespace; a != b {
		return a < b
	}
	if a, b := oi.ControlPlane.Name, oj.ControlPlane.Name; a != b {
		return a < b
	}
	if a, b := oi.Namespace, oj.Namespace; a != b {
		return a < b
	}
	if a, b := oi.Name, oj.Name; a != b {
		return a < b
	}

	return false
}
func (o ObjectsOrder) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

type EventsOrder []Event

func (e EventsOrder) Len() int { return len(e) }
func (e EventsOrder) Less(i, j int) bool {
	return e[i].LastTimestamp.Before(&e[j].LastTimestamp)
}
func (e EventsOrder) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
