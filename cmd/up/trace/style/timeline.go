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

package style

import (
	"github.com/gdamore/tcell/v2"
)

type TimeLine struct {
	Earlier tcell.Style
	Later   tcell.Style

	NotExisting tcell.Style
	Deleted     tcell.Style
	NotSynced   tcell.Style
	NotReady    tcell.Style
	Ready       tcell.Style
}

var (
	DefaultTimelineStyle = TimeLine{
		Earlier: tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),
		Later:   tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),

		NotExisting: tcell.StyleDefault,
		Deleted:     tcell.StyleDefault.Background(tcell.GetColor("#303030")),
		NotSynced:   tcell.StyleDefault.Background(tcell.GetColor("#805056")),
		NotReady:    tcell.StyleDefault.Background(tcell.GetColor("#80672c")),
		Ready:       tcell.StyleDefault.Background(tcell.GetColor("#0c7568")),
	}

	SelectedTimeLineStyle = TimeLine{
		Earlier: tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),
		Later:   tcell.StyleDefault.Foreground(tcell.GetColor("#17e1cf")),

		NotExisting: tcell.StyleDefault,
		Deleted:     tcell.StyleDefault.Background(tcell.GetColor("#737373")),
		NotSynced:   tcell.StyleDefault.Background(tcell.GetColor("#996067")),
		NotReady:    tcell.StyleDefault.Background(tcell.GetColor("#997b34")),
		Ready:       tcell.StyleDefault.Background(tcell.GetColor("#0e8c7c")),
	}
)
