package migration

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
