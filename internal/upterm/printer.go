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
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"

	"gopkg.in/yaml.v3"
)

// Printer describes interactions for working with the ObjectPrinter below.
// NOTE(tnthornton) ideally this would be called "ObjectPrinter".
// TODO(tnthornton) rename this to ObjectPrinter.
type Printer interface {
	Print(obj any, fieldNames []string, extractFields func(any) []string) error
}

// The ObjectPrinter is intended to make it easy to print individual structs
// and lists of structs for the 'get' and 'list' commands. It can print as
// a human-readable table, or computer-readable (JSON or YAML)
type ObjectPrinter struct {
	Quiet  config.QuietFlag
	Pretty bool
	Format config.Format

	TablePrinter *pterm.TablePrinter
}

var (
	DefaultObjPrinter = ObjectPrinter{
		Quiet:        false,
		Pretty:       false,
		Format:       config.Default,
		TablePrinter: pterm.DefaultTable.WithSeparator("   "),
	}
)

func init() {
	pterm.DisableStyling()
}

// Print will print a single option or an array/slice of objects.
// When printing with default table output, it will only print a given set
// of fields. To specify those fields, the caller should provide the human-readable
// names for those fields (used for column headers) and a function that can be called
// on a single struct that returns those fields as strings.
// When printing JSON or YAML, this will print *all* fields, regardless of
// the list of fields.
func (p *ObjectPrinter) Print(obj any, fieldNames []string, extractFields func(any) []string) error {
	// Step 1: If user specified quiet, skip printing entirely
	if p.Quiet {
		return nil
	}

	// Step 2: Enable color printing if desired. Note: This is only
	// implemented for the default table printing, not JSON or YAML.
	if p.Pretty {
		pterm.EnableStyling()
	}

	// Step 3: Print the object with the appropriate formatting.
	switch p.Format { //nolint:exhaustive
	case config.JSON:
		return printJSON(obj)
	case config.YAML:
		return printYAML(obj)
	default:
		return p.printDefault(obj, fieldNames, extractFields)
	}
}

func printJSON(obj any) error {
	js, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(js))
	return err
}

func printYAML(obj any) error {
	ys, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(ys))
	return err
}

func (p *ObjectPrinter) printDefault(obj any, fieldNames []string, extractFields func(any) []string) error {
	t := reflect.TypeOf(obj)
	k := t.Kind()
	if k == reflect.Array || k == reflect.Slice {
		return p.printDefaultList(obj, fieldNames, extractFields)
	}
	return p.printDefaultObj(obj, fieldNames, extractFields)
}

func (p *ObjectPrinter) printDefaultList(obj any, fieldNames []string, extractFields func(any) []string) error {
	s := reflect.ValueOf(obj)
	l := s.Len()

	data := make([][]string, l+1)
	data[0] = fieldNames
	for i := 0; i < l; i++ {
		data[i+1] = extractFields(s.Index(i).Interface())
	}
	return p.TablePrinter.WithHasHeader().WithData(data).Render()
}

func (p *ObjectPrinter) printDefaultObj(obj any, fieldNames []string, extractFields func(any) []string) error {
	data := make([][]string, 2)
	data[0] = fieldNames
	data[1] = extractFields(obj)
	return p.TablePrinter.WithHasHeader().WithData(data).Render()
}

// NewNopObjectPrinter returns a Printer that does nothing.
func NewNopObjectPrinter() Printer { return nopObjectPrinter{} }

type nopObjectPrinter struct{}

func (p nopObjectPrinter) Print(obj any, fieldNames []string, extractFields func(any) []string) error {
	return nil
}

// NewNopTextPrinter returns a TextPrinter that does nothing.
func NewNopTextPrinter() pterm.TextPrinter { return nopTextPrinter{} }

type nopTextPrinter struct{}

func (p nopTextPrinter) Sprint(a ...interface{}) string                   { return "" }
func (p nopTextPrinter) Sprintln(a ...interface{}) string                 { return "" }
func (p nopTextPrinter) Sprintf(format string, a ...interface{}) string   { return "" }
func (p nopTextPrinter) Sprintfln(format string, a ...interface{}) string { return "" }
func (p nopTextPrinter) Print(a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
func (p nopTextPrinter) Println(a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
func (p nopTextPrinter) Printf(format string, a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
func (p nopTextPrinter) Printfln(format string, a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
func (p nopTextPrinter) PrintOnError(a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
func (p nopTextPrinter) PrintOnErrorf(format string, a ...interface{}) *pterm.TextPrinter {
	tp := pterm.TextPrinter(nopTextPrinter{})
	return &tp
}
