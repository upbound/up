package dev

import (
	"github.com/spf13/afero"
)

// runCmd runs a local control plane.
type runCmd struct {
	fs afero.Fs
}
