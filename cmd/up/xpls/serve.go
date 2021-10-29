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

package xpls

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/parser"
)

var (
	re   []byte
	err  error
	pkg  *parser.Package
	root string
)

type serveCmd struct {
}

// Run --
func (c serveCmd) Run() error { // nolint:gocyclo
	log.Println("xpls is listening on stdio.")
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	fs := afero.NewOsFs()
	parse := parser.NewParser(fs)
	codec := jsonrpc2.VSCodeObjectCodec{}
	v := &jsonrpc2.Request{}
	for {
		log.Print("++++++++++++++++++++")
		if err := codec.ReadObject(reader, v); err != nil {
			log.Print(err)
		}
		log.Printf("v: %+v\n", v)
		log.Printf("v.params: %+v\n", string(*v.Params))
		log.Print(v.Method)
		switch v.Method {
		case "initialize":
			p := &lsp.InitializeParams{}
			if err := json.Unmarshal(*v.Params, p); err != nil {
				log.Print(err)
			}
			root = p.RootPath
			pkg, err = parse.ParsePackage(root)
			if err != nil {
				log.Print(err)
			}
			kind := lsp.TDSKIncremental
			re, err = json.Marshal(lsp.InitializeResult{
				Capabilities: lsp.ServerCapabilities{
					TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
						Kind: &kind,
					},
				},
			})
			if err != nil {
				log.Print(err)
			}
			reb := json.RawMessage(re)
			codec.WriteObject(writer, &jsonrpc2.Response{ // nolint:errcheck
				ID:     v.ID,
				Result: &reb,
			})
			writer.Flush() //nolint:errcheck,gosec
		case "textDocument/didOpen":
			var params lsp.DidOpenTextDocumentParams
			if err := json.Unmarshal(*v.Params, &params); err != nil {
				log.Print(err)
				continue
			}
			pkg, err = parse.ParsePackage(root)
			if err != nil {
				log.Print(err)
			}
			d, err := checkCompositionFrom(parse, pkg, params.TextDocument.URI)
			if err != nil {
				continue
			}
			re, err = json.Marshal(&lsp.PublishDiagnosticsParams{
				URI:         params.TextDocument.URI,
				Diagnostics: []lsp.Diagnostic{d},
			})
			if err != nil {
				log.Print(err)
			}
		case "textDocument/didChange":
			// TODO(hasheddan): reading the file before save will not reveal
			// unsaved state, so parsing package again here is redundant.
			var params lsp.DidChangeTextDocumentParams
			if err := json.Unmarshal(*v.Params, &params); err != nil {
				log.Print(err)
				continue
			}
			pkg, err = parse.ParsePackage(root)
			if err != nil {
				log.Print(err)
			}
			d, err := checkCompositionFrom(parse, pkg, params.TextDocument.URI)
			if err != nil {
				log.Print(err)
				continue
			}
			// d := lsp.Diagnostic{
			// 	Range: lsp.Range{
			// 		Start: lsp.Position{
			// 			Line:      params.ContentChanges[0].Range.Start.Line,
			// 			Character: params.ContentChanges[0].Range.Start.Character,
			// 		},
			// 		End: lsp.Position{
			// 			Line:      params.ContentChanges[0].Range.End.Line,
			// 			Character: params.ContentChanges[0].Range.End.Character,
			// 		},
			// 	},
			// 	Severity: lsp.Error,
			// 	Source:   "xpls",
			// 	Message:  fmt.Sprintf("InfrastructureDefinition %s is undefined", "test"),
			// }
			log.Print(d)
			re, err = json.Marshal(&lsp.PublishDiagnosticsParams{
				URI:         params.TextDocument.URI,
				Diagnostics: []lsp.Diagnostic{d},
			})
			if err != nil {
				log.Print(err)
			}
		case "textDocument/didClose":
			var params lsp.DidCloseTextDocumentParams
			if err := json.Unmarshal(*v.Params, &params); err != nil {
				log.Print(err)
				continue
			}
			pkg, err = parse.ParsePackage(root)
			if err != nil {
				log.Print(err)
			}
			d, err := checkCompositionFrom(parse, pkg, params.TextDocument.URI)
			if err != nil {
				log.Print(err)
				continue
			}
			re, err = json.Marshal(&lsp.PublishDiagnosticsParams{
				URI:         params.TextDocument.URI,
				Diagnostics: []lsp.Diagnostic{d},
			})
			if err != nil {
				log.Print(err)
			}
		case "textDocument/didSave":
			var params lsp.DidSaveTextDocumentParams
			if err := json.Unmarshal(*v.Params, &params); err != nil {
				log.Print(err)
				continue
			}
			pkg, err = parse.ParsePackage(root)
			if err != nil {
				log.Print(err)
			}
			d, err := checkCompositionFrom(parse, pkg, params.TextDocument.URI)
			if err != nil {
				log.Print(err)
				continue
			}
			re, err = json.Marshal(&lsp.PublishDiagnosticsParams{
				URI:         params.TextDocument.URI,
				Diagnostics: []lsp.Diagnostic{d},
			})
			if err != nil {
				log.Print(err)
			}
		default:
			continue
		}
		reb := json.RawMessage(re)
		codec.WriteObject(writer, &jsonrpc2.Request{ // nolint:errcheck
			Method: "textDocument/publishDiagnostics",
			ID:     v.ID,
			Notif:  true,
			Params: &reb,
		})
		writer.Flush() //nolint:errcheck,gosec
	}
}

func stripFilePrefix(uri lsp.DocumentURI) string {
	return strings.TrimPrefix(string(uri), "file://")
}

func checkCompositionFrom(parse *parser.Parser, pkg *parser.Package, uri lsp.DocumentURI) (lsp.Diagnostic, error) {
	f, ok := pkg.Compositions[stripFilePrefix(uri)]
	if !ok {
		return lsp.Diagnostic{}, errors.New("file is not a Composition")
	}
	idExists := false
	// for _, id := range pkg.InfrastructureDefinitions {
	// 	if id.Spec.CRDSpecTemplate.Names.Kind == f.Spec.From.Kind && fmt.Sprintf("%s/%s", id.Spec.CRDSpecTemplate.Group, id.Spec.CRDSpecTemplate.Version) == f.Spec.From.APIVersion {
	// 		idExists = true
	// 		break
	// 	}
	// }
	if idExists {
		return lsp.Diagnostic{}, nil
	}
	start, end, err := parse.ParseLines(stripFilePrefix(uri), "from:", "kind:")
	if err != nil {
		return lsp.Diagnostic{}, err
	}
	return lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      start,
				Character: 0,
			},
			End: lsp.Position{
				Line:      end + 1,
				Character: 0,
			},
		},
		Severity: lsp.Error,
		Source:   "xpls",
		Message:  fmt.Sprintf("CompositeTypeRef %s is undefined", f.Spec.CompositeTypeRef.Kind),
	}, nil
}
