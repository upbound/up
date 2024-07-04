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

package mirror

import (
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/upterm"
)

// DefaultSpinner is the default spinner used by all the other subpackages, by
// default it's just a no-op.
var DefaultSpinner Spinner = noopSpinner{}

// noopSpinner is a spinner that does nothing.
type noopSpinner struct{}

func (noopSpinner) Start(_ ...interface{}) (Printer, error) {
	return &noopPrinter{}, nil
}

// noopPrinter is a spinner printer that does nothing.
type noopPrinter struct{}

func (n noopPrinter) Success(_ ...interface{}) {}

func (n noopPrinter) Fail(_ ...interface{}) {}

func (n noopPrinter) UpdateText(_ string) {}

// Spinner is an interface for creating Printers.
type Spinner interface {
	Start(text ...interface{}) (Printer, error)
}

// Printer is an interface for printing through a spinner.
type Printer interface {
	Success(msg ...interface{})
	Fail(msg ...interface{})
	UpdateText(text string)
}

// setupStyling to enable or disable styling for dry-run disabled.
func setupStyling(printer upterm.ObjectPrinter) {
	if !printer.DryRun {
		pterm.EnableStyling()
	} else {
		pterm.DisableStyling()
	}
}

// logAndStartSpinner to enable or disable spinner for dry-run disabled.
func logAndStartSpinner(printer upterm.ObjectPrinter, message string) *pterm.SpinnerPrinter {
	if printer.DryRun {
		return nil
	}
	spinnerInstance, _ := DefaultSpinner.Start(message)
	return spinnerInstance.(*pterm.SpinnerPrinter)
}
