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

package views

import (
	"fmt"

	"github.com/rivo/tview"
	"k8s.io/apimachinery/pkg/types"

	"github.com/upbound/up/cmd/up/trace/model"
)

type YAML struct {
	*tview.TextView
}

func NewYAML(o *model.Object, s string) *YAML {
	y := &YAML{
		TextView: tview.NewTextView().
			SetDynamicColors(true).
			SetText(s),
	}
	y.TextView.SetBorder(true).
		SetTitle(fmt.Sprintf(" [::b]%s[::-] %s [darkgray]in ControlPlane[-] %s/%s ", o.Kind, types.NamespacedName{Namespace: o.Namespace, Name: o.Name}, o.ControlPlane.Namespace, o.ControlPlane.Name))

	return y
}
