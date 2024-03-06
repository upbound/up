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

package template

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/tview-template/model"
	"github.com/upbound/up/cmd/up/tview-template/views"
	"github.com/upbound/up/internal/tview/dialogs"
	upviews "github.com/upbound/up/internal/tview/views"
)

// App represents main application struct.
type App struct {
	*tview.Application
	model model.App

	header  *views.Header
	example *views.Example

	grid     *tview.Grid
	topLevel *upviews.TopLevel
}

func NewApp(title string) *App {
	app := &App{
		Application: tview.NewApplication(),
		model:       model.NewApp(),
	}

	app.header = views.NewHeader()
	app.example = views.NewExample()

	app.grid = tview.NewGrid().
		SetRows(1, 0).
		SetColumns(0).
		SetBorders(true).
		SetBordersColor(tcell.ColorDarkGray).
		AddItem(app.header, 0, 0, 1, 1, 0, 0, false).
		AddItem(app.example, 1, 0, 1, 1, 0, 0, true)
	app.topLevel = upviews.NewTopLevel(title, app.grid, app.Application).
		SetTitles(upviews.GridTitle{Col: 0, Row: 1, Text: " Example ", Color: tcell.ColorDarkGray, Align: tview.AlignCenter}).
		SetDelegateInputHandler(app.TopLevelInputHandler).
		SetCommands("", "Example", "", "", "", "", "", "", "", "Quit")
	app.Application.SetRoot(app.topLevel, true)

	return app
}

func (a *App) TopLevelInputHandler(event *tcell.EventKey, setFocus func(p tview.Primitive)) bool {
	switch event.Key() {
	case tcell.KeyF2:
		oldRoot := dialogs.GetRoot(a.Application)
		dialogs.ShowModal(a.Application, dialogs.NewConfirmDialog().
			SetTitle("Congratulations").
			SetText("You pressed F2 🎉").
			SetCancelFunc(func() { a.Application.SetRoot(oldRoot, true) }).
			SetSelectedFunc(func() { a.Application.SetRoot(oldRoot, true) }).
			Display())
		return true
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			a.topLevel.InteractiveQuit()
		}
	default:
	}

	return false
}

func (a *App) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		a.Application.Stop()
	}()

	go func() {
		for {
			time.Sleep(time.Second)
			a.QueueUpdateDraw(func() {})
		}
	}()

	return a.Application.Run()
}
