package upbound

import "github.com/pterm/pterm"

var (
	eyesPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 👀",
	}

	raisedPrefix = pterm.Prefix{
		Style: &pterm.Style{pterm.FgLightMagenta},
		Text:  " 🙌",
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
		Prefix:       eyesPrefix,
	}

	checkmarkSuccessSpinner = pterm.DefaultSpinner.WithStyle(spinnerStyle)
	eyesInfoSpinner         = pterm.DefaultSpinner.WithStyle(spinnerStyle)

	componentText = pterm.DefaultBasicText.WithStyle(&pterm.ThemeDefault.TreeTextStyle)
)

func init() {
	checkmarkSuccessSpinner.SuccessPrinter = cp
	eyesInfoSpinner.InfoPrinter = ip
}

func wrapWithSuccessSpinner(msg string, spinner *pterm.SpinnerPrinter, f func() error) error {
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

func wrapWithInfoSpinner(msg string, spinner *pterm.SpinnerPrinter, f func() error) error {
	s, err := spinner.Start(msg)
	if err != nil {
		return err
	}

	if err := f(); err != nil {
		return err
	}

	s.Info()
	return nil
}
