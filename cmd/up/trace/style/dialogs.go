package style

import (
	"github.com/gdamore/tcell/v2"
)

var (
	DialogFgColor = tcell.ColorDefault
	DialogBgColor = tcell.NewRGBColor(98, 0, 140)

	ButtonBgColor     = tcell.Color241
	DialogBorderColor = tcell.ColorWhite
)

const (
	// DialogPadding dialog inner paddign.
	DialogPadding = 3
	// DialogFormHeight dialog "Enter"/"Cancel" form height.
	DialogFormHeight = 3

	// DialogMinWidth dialog min width.
	DialogMinWidth = 40

	InputFieldBgColor = tcell.ColorLightGray
	InputFieldFgColor = tcell.ColorBlack
)
