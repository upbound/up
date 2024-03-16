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

package trace

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/duration"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	queryv1alpha1 "github.com/upbound/up-sdk-go/apis/query/v1alpha1"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/trace/model"
	"github.com/upbound/up/cmd/up/trace/views"
	"github.com/upbound/up/internal/tview/dialogs"
	upviews "github.com/upbound/up/internal/tview/views"
)

// App represents main application struct.
type App struct {
	*tview.Application
	model *model.App

	header   *views.Header
	tree     *views.Tree
	timeline *views.TimeLine
	status   *views.Status
	details  *views.Details

	grid     *tview.Grid
	topLevel *upviews.TopLevel

	pollFn  func(gkns query.GroupKindNames, cns query.CategoryNames) ([]queryv1alpha1.QueryResponseObject, error)
	fetchFn func(id string) (*unstructured.Unstructured, error)
}

func NewApp(title string, resources []string, gkns query.GroupKindNames, cns query.CategoryNames, pollFn func(gkns query.GroupKindNames, cns query.CategoryNames) ([]queryv1alpha1.QueryResponseObject, error), fetchFn func(id string) (*unstructured.Unstructured, error)) *App {
	app := &App{
		Application: tview.NewApplication(),
		model:       model.NewApp(resources, gkns, cns),
		pollFn:      pollFn,
		fetchFn:     fetchFn,
	}

	app.header = views.NewHeader()
	app.tree = views.NewTree(app.Application, &app.model.Tree)
	app.timeline = views.NewTimeLine(app.tree, app.model)
	app.status = views.NewStatus(app.tree)
	app.details = views.NewDetails(app.tree)

	app.tree.AddScroller(app.timeline, app.status)

	app.grid = tview.NewGrid().
		SetRows(1, 0, 4).
		SetBorders(true).
		SetBordersColor(tcell.ColorDarkGray).
		SetColumns(40, 0, 75).
		AddItem(app.header, 0, 0, 1, 3, 0, 0, false).
		AddItem(app.tree, 1, 0, 1, 1, 0, 0, true).
		AddItem(app.timeline, 1, 1, 1, 1, 0, 0, true).
		AddItem(app.details, 2, 0, 1, 3, 0, 0, false)
	app.Unzoom()
	app.topLevel = upviews.NewTopLevel(title, app.grid, app.Application).
		SetTitles(
			upviews.GridTitle{Col: 1, Row: 1, Fn: func(screen tcell.Screen, x0, y, w int) {
				ts := app.model.TimeLine.Scale
				if !app.model.TimeLine.FixedTime.IsZero() {
					ts += time.Since(app.model.TimeLine.FixedTime.Truncate(app.model.TimeLine.Scale / 10))
				}
				for x := x0 + w - 14; x > x0-4; x -= 10 {
					tview.Print(screen, duration.HumanDuration(ts), x, y, 9, tview.AlignCenter, tcell.ColorDarkGray)
					ts += app.model.TimeLine.Scale
				}
				if app.model.TimeLine.FixedTime.IsZero() {
					tview.Print(screen, "Now", x0+w-2, y, 3, tview.AlignLeft, tcell.ColorDarkGray)
				} else {
					screen.SetContent(x0+w, y, '>', nil, tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
				}
			}},
			upviews.GridTitle{Col: 2, Row: 1, Text: "── Progress ── Synced Ready  Message ", Color: tcell.ColorDarkGray, Align: tview.AlignLeft},
			upviews.GridTitle{Col: 0, Row: 2, Text: " Details ", Color: tcell.ColorDarkGray, Align: tview.AlignCenter},
			upviews.GridTitle{Col: 0, Row: 2, Fn: func(screen tcell.Screen, x, y, w int) {
				var b strings.Builder
				if app.model.Tree.AutoCollapse {
					b.WriteString("AutoCollapse ")
				}
				if app.model.Zoomed {
					b.WriteString("Zoomed ")
				}

				if b.Len() > 0 {
					tview.Print(screen, " [::b]"+b.String(), x, y, w, tview.AlignRight, tcell.ColorYellow)
				}
			}},
		).
		SetError(app.model.TopLevel.Error).
		SetCommands("Help", "Kind", "View", "", "", "", "", "", "", "Quit").
		SetDelegateInputHandler(app.TopLevelInputHandler)
	app.Application.SetRoot(app.topLevel, true)
	app.Application.SetFocus(app.tree)

	return app
}

func (a *App) Zoom() {
	a.grid.RemoveItem(a.timeline)
	a.grid.AddItem(a.timeline, 1, 1, 1, 2, 0, 0, true)
	a.grid.RemoveItem(a.status)
	a.model.Zoomed = true
}

func (a *App) Unzoom() {
	a.grid.AddItem(a.timeline, 1, 1, 1, 1, 0, 0, true)
	a.grid.AddItem(a.status, 1, 2, 1, 1, 0, 0, false)
	a.model.Zoomed = false
}

func (a *App) TopLevelInputHandler(event *tcell.EventKey, setFocus func(p tview.Primitive)) bool { // nolint:gocyclo // TODO: split up
	switch event.Key() { // nolint:exhaustive // there is a default case
	case tcell.KeyEscape:
		if a.model.Zoomed {
			a.Unzoom()
			return true
		}
	case tcell.KeyUp:
		if a.GetFocus() == a.topLevel || a.GetFocus() == a.tree {
			a.tree.InputHandler()(event, setFocus)
			return true
		}
	case tcell.KeyDown:
		if a.GetFocus() == a.topLevel || a.GetFocus() == a.tree {
			a.tree.InputHandler()(event, setFocus)
			return true
		}
	case tcell.KeyLeft:
		if a.model.TimeLine.FixedTime.IsZero() {
			a.model.TimeLine.FixedTime = time.Now()
		}
		a.model.TimeLine.FixedTime = a.model.TimeLine.FixedTime.Add(-a.model.TimeLine.Scale / 10)
		return true
	case tcell.KeyRight:
		if a.model.TimeLine.FixedTime.IsZero() {
			a.model.TimeLine.FixedTime = time.Now()
		}
		a.model.TimeLine.FixedTime = a.model.TimeLine.FixedTime.Add(a.model.TimeLine.Scale / 10)
		if a.model.TimeLine.FixedTime.After(time.Now()) {
			a.model.TimeLine.FixedTime = time.Time{} // back to auto-scrolling
		}
		return true
	case tcell.KeyEnd:
		a.model.TimeLine.FixedTime = time.Time{}
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			a.topLevel.InteractiveQuit()
		case 'f':
			if a.model.Zoomed {
				a.Unzoom()
			} else {
				a.Zoom()
			}
			return true
		case 'T':
			for i, scale := range model.Scales {
				if scale == a.model.TimeLine.Scale {
					if i < len(model.Scales)-1 {
						a.model.TimeLine.Scale = model.Scales[i+1]
					}
					break
				}
			}
			return true
		case 't':
			for i, scale := range model.Scales {
				if scale == a.model.TimeLine.Scale {
					if i > 0 {
						a.model.TimeLine.Scale = model.Scales[i-1]
					}
					break
				}
			}
			return true
		default:
		}
	case tcell.KeyF2:
		resources := *a.model.Resources.Load()

		oldRoot := dialogs.GetRoot(a.Application)
		dlg := dialogs.NewSimpleInputDialog(strings.Join(resources, " ")).
			SetTitle("Types").
			SetLabel("Kind").
			SetDescription("Format: TYPE[.GROUP][,TYPE[.GROUP]...] [NAME ...] | TYPE[.GROUP]/NAME ...").
			SetCancelFunc(func() { a.SetRoot(oldRoot, true) }).
			SetSelectedFunc(func(value string) {
				value = strings.TrimSpace(value)
				if value == "" {
					a.model.TopLevel.SetError(errors.New("No resources specified"))
					return
				}

				resources := strings.Split(value, " ")
				tgns, errs := query.ParseTypesAndNames(resources...)
				if err := kerrors.NewAggregate(errs); err != nil {
					a.model.TopLevel.SetError(err)
					return
				}
				gkns, cns := query.SplitGroupKindAndCategories(tgns)

				a.model.Resources.Store(&resources)
				a.model.GroupKindNames.Store(&gkns)
				a.model.CategoryNames.Store(&cns)

				a.SetRoot(oldRoot, true)
			})
		dialogs.ShowModal(a.Application, dlg.Display())

		return true
	case tcell.KeyF3:
		n := a.tree.GetCurrentNode()
		if n == nil {
			break
		}
		ref := n.GetReference()
		if ref == nil {
			return false
		}
		o := ref.(*model.Object)

		obj, err := a.fetchFn(o.Id)
		if err != nil {
			return false
		}

		txt := views.NewYAML(o, renderYAML(obj)).
			SetChangedFunc(func() { a.Draw() }).
			SetDoneFunc(func(key tcell.Key) { a.SetRoot(a.topLevel, true) })
		a.ResizeToFullScreen(txt)
		a.SetRoot(txt, true)

		return true
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

	resp, err := a.pollFn(*a.model.GroupKindNames.Load(), *a.model.CategoryNames.Load())
	if err != nil {
		return err
	}

	a.model.Tree.Update(resp)

	go func() {
		for {
			time.Sleep(time.Second * 1)
			objs, err := a.pollFn(*a.model.GroupKindNames.Load(), *a.model.CategoryNames.Load())
			if err != nil {
				a.model.TopLevel.SetError(errors.Errorf(" Error: %v ", err))
				continue
			}

			a.QueueUpdateDraw(func() {
				a.model.Tree.Update(objs)
			})
		}
	}()

	return a.Application.Run()
}

func renderYAML(obj *unstructured.Unstructured) string {
	obj = obj.DeepCopy()

	if fld, ok := obj.Object["metadata"]; ok {
		if metadata, ok := fld.(map[string]interface{}); ok {
			delete(metadata, "managedFields")
		}
	}

	bs, err := yaml.Marshal(obj)
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	if err := quick.Highlight(&b, string(bs), "yaml", "terminal16m", "monokai"); err != nil {
		return ""
	}

	return tview.TranslateANSI(b.String())
}
