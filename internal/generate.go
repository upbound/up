// +build generate

// NOTE(negz): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Add license headers to all files.
//go:generate go run -tags generate github.com/google/addlicense -v -c "Upbound Inc" . ../cmd

package internal

import (
	_ "github.com/google/addlicense" //nolint:typecheck
)
