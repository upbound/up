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
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/trace/style"
)

type Header struct {
	*tview.TextView
}

func NewHeader() *Header {
	return &Header{
		TextView: tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetText(" ↑↓ up/down   ←→ time   +- expand/collapse   enter,space toggle   a auto-collapse   tab focus   f zoom   t,T time-scale   F3 yaml   end now   q,F10 quit").
			SetTextColor(style.Header),
	}
}
