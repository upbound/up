// Copyright 2024 Upbound Inc
// Copyright 2023 podman-tui authors
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

package dialogs

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/trace/style"
)

// ConfirmDialog is a simple confirmation dialog primitive.
type ConfirmDialog struct {
	*tview.Box
	layout        *tview.Flex
	textview      *tview.TextView
	form          *tview.Form
	x             int
	y             int
	width         int
	height        int
	message       string
	display       bool
	cancelHandler func()
	selectHandler func()
}

// NewConfirmDialog returns new confirm dialog primitive.
func NewConfirmDialog() *ConfirmDialog {
	dialog := &ConfirmDialog{
		Box:           tview.NewBox(),
		display:       false,
		cancelHandler: func() {},
		selectHandler: func() {},
	}

	dialog.textview = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetTextAlign(tview.AlignLeft)

	dialog.textview.SetBackgroundColor(style.DialogBgColor)
	dialog.textview.SetTextColor(style.DialogFgColor)

	dialog.form = tview.NewForm().
		AddButton("Cancel", nil).
		AddButton("  OK  ", nil).
		SetButtonsAlign(tview.AlignRight)
	dialog.form.SetBackgroundColor(style.DialogBgColor)
	dialog.form.SetButtonBackgroundColor(style.ButtonBgColor)

	dialog.layout = tview.NewFlex().SetDirection(tview.FlexRow)
	dialog.layout.SetBorder(true)
	dialog.layout.SetBorderColor(style.DialogBorderColor)
	dialog.layout.SetBackgroundColor(style.DialogBgColor)

	return dialog
}

// Display displays this primitive.
func (d *ConfirmDialog) Display() *ConfirmDialog {
	d.form.SetFocus(1)
	d.display = true
	return d
}

// IsDisplay returns true if primitive is shown.
func (d *ConfirmDialog) IsDisplay() bool {
	return d.display
}

// Hide stops displaying this primitive.
func (d *ConfirmDialog) Hide() *ConfirmDialog {
	d.textview.SetText("")
	d.message = ""
	d.display = false
	return d
}

// SetTitle sets dialog title.
func (d *ConfirmDialog) SetTitle(title string) *ConfirmDialog {
	d.layout.SetTitle(strings.ToUpper(title))
	d.layout.SetTitleColor(style.DialogFgColor)
	return d
}

// SetText sets dialog title.
func (d *ConfirmDialog) SetText(message string) *ConfirmDialog {
	d.message = message
	d.textview.Clear()

	msg := "\n" + message

	d.textview.SetText(msg)
	d.textview.ScrollToBeginning()
	d.setRect()

	return d
}

// Focus is called when this primitive receives focus.
func (d *ConfirmDialog) Focus(delegate func(p tview.Primitive)) {
	delegate(d.form)
}

// HasFocus returns whether or not this primitive has focus.
func (d *ConfirmDialog) HasFocus() bool {
	return d.form.HasFocus()
}

// SetRect set rects for this primitive.
func (d *ConfirmDialog) SetRect(x, y, width, height int) {
	d.x = x + style.DialogPadding
	d.y = y + style.DialogPadding
	d.width = width - (2 * style.DialogPadding)   //nolint:gomnd
	d.height = height - (2 * style.DialogPadding) //nolint:gomnd
	d.setRect()
}

func (d *ConfirmDialog) setRect() {
	maxHeight := d.height
	maxWidth := d.width
	messageHeight := len(strings.Split(d.message, "\n"))
	messageWidth := getMessageWidth(d.message)

	layoutHeight := messageHeight + 2 //nolint:gomnd

	if maxHeight > layoutHeight+style.DialogFormHeight {
		d.height = layoutHeight + style.DialogFormHeight + 2 //nolint:gomnd
	} else {
		d.height = maxHeight
		layoutHeight = d.height - style.DialogFormHeight - 2 //nolint:gomnd
	}

	if maxHeight > d.height {
		emptyHeight := (maxHeight - d.height) / 2 //nolint:gomnd
		d.y += emptyHeight
	}

	if d.width > style.DialogMinWidth {
		if messageWidth < style.DialogMinWidth {
			d.width = style.DialogMinWidth + 2 //nolint:gomnd
		} else if messageWidth < d.width {
			d.width = messageWidth + 2 //nolint:gomnd
		}
	}

	if maxWidth > d.width {
		emptyWidth := (maxWidth - d.width) / 2 //nolint:gomnd
		d.x += emptyWidth
	}

	msgLayout := tview.NewFlex().SetDirection(tview.FlexColumn)
	msgLayout.AddItem(emptyBoxSpace(style.DialogBgColor), 1, 0, false)
	msgLayout.AddItem(d.textview, 0, 1, true)
	msgLayout.AddItem(emptyBoxSpace(style.DialogBgColor), 1, 0, false)

	d.layout.Clear()
	d.layout.AddItem(msgLayout, layoutHeight, 0, true)
	d.layout.AddItem(d.form, style.DialogFormHeight, 0, true)

	d.Box.SetRect(d.x, d.y, d.width, d.height)
}

// Draw draws this primitive onto the screen.
func (d *ConfirmDialog) Draw(screen tcell.Screen) {
	if !d.display {
		return
	}

	d.Box.DrawForSubclass(screen, d)

	x, y, width, height := d.Box.GetInnerRect()
	d.layout.SetRect(x, y, width, height)
	d.layout.Draw(screen)
}

// InputHandler returns input handler function for this primitive.
func (d *ConfirmDialog) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return d.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEsc {
			d.cancelHandler()
			return
		}

		if formHandler := d.form.InputHandler(); formHandler != nil {
			formHandler(event, setFocus)
			return
		}
	})
}

// SetCancelFunc sets form cancel button selected function.
func (d *ConfirmDialog) SetCancelFunc(handler func()) *ConfirmDialog {
	d.cancelHandler = handler
	cancelButton := d.form.GetButton(d.form.GetButtonCount() - 2) //nolint:gomnd
	cancelButton.SetSelectedFunc(handler)

	return d
}

// SetSelectedFunc sets form select button selected function.
func (d *ConfirmDialog) SetSelectedFunc(handler func()) *ConfirmDialog {
	d.selectHandler = handler
	enterButton := d.form.GetButton(d.form.GetButtonCount() - 1)
	enterButton.SetSelectedFunc(handler)

	return d
}

func getMessageWidth(message string) int {
	var messageWidth int

	for _, msg := range strings.Split(message, "\n") {
		if len(msg) > messageWidth {
			messageWidth = len(msg)
		}
	}

	return messageWidth
}

// emptyBoxSpace returns simple Box without border with bgColor as background.
func emptyBoxSpace(bgColor tcell.Color) *tview.Box {
	box := tview.NewBox()
	box.SetBackgroundColor(bgColor)
	box.SetBorder(false)

	return box
}
