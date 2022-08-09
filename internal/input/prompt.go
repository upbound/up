// Copyright 2021 Upbound Inc
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

package input

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/term"
)

const (
	errNotTTY = "refusing to prompt in non-interactive terminal"

	newLine = '\n'
)

// file is a convenience wrapper for special files, such as stdin and stdout.
type file interface {
	io.Reader
	io.Writer
	Fd() uintptr
}

// tty performs operations on interactive terminals.
type tty interface {
	IsTerminal(int) bool
	ReadPassword(int) ([]byte, error)
}

type defaultTTY struct{}

func (defaultTTY) IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func (defaultTTY) ReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

// Prompter prompts a user for input.
type Prompter interface {
	Prompt(label string, sensitive bool) (string, error)
}

// NewPrompter constructs a new prompter that uses stdin for input and stdout
// for output.
func NewPrompter() Prompter {
	return &defaultPrompter{
		in:  os.Stdin,
		out: os.Stdout,
		tty: defaultTTY{},
	}
}

// defaultPrompter is a prompter that uses stdin for input and stdout for
// output.
type defaultPrompter struct {
	in  file
	out file
	tty tty
}

// Prompt prompts a user for input for the specified label. Input is obscured if
// sensitive is specified.
func (d *defaultPrompter) Prompt(label string, sensitive bool) (string, error) {
	if !d.tty.IsTerminal(int(d.in.Fd())) {
		return "", errors.New(errNotTTY)
	}
	if _, err := fmt.Fprintf(d.out, "\033[1;36m%s\033[0m: ", label); err != nil {
		return "", err
	}
	reader := bufio.NewReader(d.in)
	if !sensitive {
		s, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}
	b, err := d.tty.ReadPassword(int(d.in.Fd()))
	if err != nil {
		return "", err
	}
	// manually write newline since tty.ReadPassword silences echo, including
	// the user-entered newline
	if _, err := d.out.Write([]byte{newLine}); err != nil {
		return "", err
	}
	return string(b), nil
}
