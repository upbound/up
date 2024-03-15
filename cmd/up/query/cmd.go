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

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package query

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/kubectl/pkg/cmd/get"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

var printFlags = get.NewGetPrintFlags()

// QueryCmd contains commands for querying control plane objects.
type cmd struct {
	AllResources bool `name:"all-resources" help:"Query all resources in the control plane."`

	// general printer flags
	OutputFormat string   `short:"o" name:"output" help:"Output format. One of: json,yaml,name,go-template,go-template-file,template,templatefile,jsonpath,jsonpath-as-json,jsonpath-file,custom-columns,custom-columns-file,wide"`
	NoHeaders    bool     `help:"When using the default or custom-column output format, don't print headers."`
	ShowLabels   bool     `name:"show-labels" help:"When printing, show all labels as the last column (default hide labels column)"`
	SortBy       string   `name:"sort-by" help:"If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string."`
	ColumnLabels []string `name:"label-columns" help:"Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2..."`
	ShowKind     bool     `name:"show-kind" help:"If present, list the resource type for the requested object(s)."`

	// template printer flags
	Template         string `short:"t" name:"template" help:"Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]."`
	AllowMissingKeys bool   `name:"allow-missing-template-keys" help:"If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats."`

	// json/yaml flags
	ShowManagedFields bool `name:"show-managed-fields" help:"If true, keep the managedFields when printing objects in JSON or YAML format."`

	// positional arguments
	Resources []string `arg:"" optional:"" help:"Type(s) (resource, singular or plural, category, short-name) and names: TYPE[.GROUP][,TYPE[.GROUP]...] [NAME ...] | TYPE[.GROUP]/NAME .... If no resource is specified, all resources are queried, but --all-resources must be specified."`

	Flags upbound.Flags `embed:""`

	printFlags *get.PrintFlags
	namespace  string // inside the control plane
}

func help(cmdName string) (string, error) {
	t, err := template.New("help").Parse(`Examples:
  # List all S3 buckets in ps output format
  {{.CmdName}} buckets

  # List all buckets in ps output format with more information (such as node name)
  {{.CmdName}} buckets -o wide

  # List a single S3 bucket with specified NAME in ps output format
  {{.CmdName}} bucket web-bucket-13je7

  # List S3 buckets in JSON output format, in the "v1" version of the "s3.aws.upbound.io" API group
  {{.CmdName}} buckets.v1.s3.aws.upbound.io -o json

  # List a single bucket in JSON output format
  {{.CmdName}} -o json bucket web-bucket-13je7

  # Return only the external-name value of the specified bucket
  {{.CmdName}} -o template bucket/web-bucket-13je7 --template={{"{{.metadata.annotations.external-name}}"}}

  # List resource information in custom columns
  {{.CmdName}} bucket test-bucket -o custom-columns=NAME:.spec.forProvider.name,SIZE:.status.atProvider.size

  # List all replication controllers and services together in ps output format
  {{.CmdName}} buckets,vpcs

  # List one or more resources by their type and names
  {{.CmdName}} vpc/prod bucket/backup providerconfig/kube
`)
	if err != nil {
		return "", errors.Wrap(err, "failed to create help template")
	}

	type Data struct {
		CmdName string
		Formats string
	}

	output := new(bytes.Buffer)
	if err := t.Execute(output, Data{
		CmdName: cmdName,
		Formats: strings.Join(printFlags.AllowedFormats(), "|"),
	}); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return output.String(), nil
}

type NotFound interface {
	PrintMessage() error
}

type NotFoundFunc func() error

func (f NotFoundFunc) PrintMessage() error {
	return f()
}
