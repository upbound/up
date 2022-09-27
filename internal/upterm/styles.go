// Copyright 2022 Upbound Inc
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

package upterm

import (
	"fmt"

	"github.com/pterm/pterm"
)

var (
	EyesPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 👀",
	}

	spinnerStyle = &pterm.Style{pterm.FgDarkGray}

	cp = &pterm.PrefixPrinter{
		MessageStyle: &pterm.Style{pterm.FgLightWhite},
		Prefix: pterm.Prefix{
			Style: &pterm.Style{pterm.FgLightMagenta},
			Text:  " √ ",
		},
	}
	ip = &pterm.PrefixPrinter{
		MessageStyle: &pterm.Style{pterm.FgLightWhite},
		Prefix:       EyesPrefix,
	}

	CheckmarkSuccessSpinner = pterm.DefaultSpinner.WithStyle(spinnerStyle)
	EyesInfoSpinner         = pterm.DefaultSpinner.WithStyle(spinnerStyle)

	RaisedPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 🙌",
	}

	ComponentText = pterm.DefaultBasicText.WithStyle(&pterm.ThemeDefault.TreeTextStyle)
)

func init() {
	CheckmarkSuccessSpinner.SuccessPrinter = cp
	EyesInfoSpinner.InfoPrinter = ip
}

func WrapWithSuccessSpinner(msg string, spinner *pterm.SpinnerPrinter, f func() error) error {
	s, err := spinner.Start(msg)
	if err != nil {
		return err
	}

	if err := f(); err != nil {
		return err
	}

	s.Success()
	return nil
}

func StepCounter(msg string, index, total int) string {
	return fmt.Sprintf("[%d/%d]: %s", index, total, msg)
}
