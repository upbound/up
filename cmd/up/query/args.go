// Copyright 2024 Upbound Inc
// Copyright 2014-2024 The Kubernetes Authors.
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

package query

import (
	"fmt"
	"os"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var nativeScheme = runtime.NewScheme()

func init() {
	kruntime.Must(apiextensionsv1.AddToScheme(nativeScheme))
	kruntime.Must(apiextensionsv1beta1.AddToScheme(nativeScheme))
	kruntime.Must(clientgoscheme.AddToScheme(nativeScheme))
}

// This file mostly comes from k8s.io/cli-runtime/pkg/resource, adapted for our
// use-cases without the builder.

type typeNames struct {
	Resource string
	Names    []string
}

type typeGroupNames struct {
	Type  string
	Group string
	Names []string
}

// ParseTypesAndNames parses
//
//		``
//	 	`<type1>[,<type2>,...] [<name1> [<name2>...]]`
//	 	`<type1>/<name1> [<type2>/<name2>...]`
//
// into resource tuples. Every tuple will result in a query lateron.
func ParseTypesAndNames(args ...string) (resourceTuples []typeGroupNames, errs []error) {
	tuples, errs := parseTypesAndNames(args...)
	if len(errs) > 0 {
		return nil, errs
	}
	if len(tuples) == 0 {
		return nil, nil
	}

	// fill in groups
	groupTuples := make([]typeGroupNames, len(tuples))
	for i := range tuples {
		ss := strings.SplitN(tuples[i].Resource, ".", 2)
		if len(ss) == 1 {
			groupTuples[i] = typeGroupNames{
				Type:  tuples[i].Resource,
				Names: tuples[i].Names,
			}
			continue
		}
		groupTuples[i] = typeGroupNames{
			Type:  ss[0],
			Names: tuples[i].Names,
			Group: ss[1],
		}

		// error on versions
		ss = strings.SplitN(ss[1], ".", 2)
		if !strings.HasPrefix(ss[0], "v") || ss[0] == "v" {
			continue
		}
		if ss[0][1] >= '0' && ss[0][1] <= '9' {
			errs = append(errs, fmt.Errorf("resource may not be specified by version"))
			return nil, errs
		}
	}

	return groupTuples, nil
}

func parseTypesAndNames(args ...string) (resourceTuples []typeNames, errs []error) { // nolint:gocyclo // core algorithm, doesn't get simpler by splitting.
	if ok, err := hasCombinedTypeArgs(args); ok {
		if err != nil {
			errs = append(errs, err)
			return nil, errs
		}
		for _, s := range args {
			tuple, ok, err := splitTypeName(s)
			if err != nil {
				errs = append(errs, err)
				return nil, errs
			}
			if ok {
				resourceTuples = append(resourceTuples, tuple)
			}
		}
		return resourceTuples, nil
	}

	switch {
	case len(args) >= 1:
		for _, res := range strings.Split(args[0], ",") {
			names := args[1:]
			if len(names) == 0 {
				names = nil
			}
			for _, n := range names {
				if strings.Contains(n, ",") {
					errs = append(errs, fmt.Errorf("names may not be comma-separated, use spaces"))
					return nil, errs
				}
			}
			resourceTuples = append(resourceTuples, typeNames{Resource: res, Names: names})
		}
	case len(args) == 0:
	default:
		errs = append(errs, fmt.Errorf("arguments must consist of a resource, a resource and name or resource/name"))
	}
	return resourceTuples, errs
}

func hasCombinedTypeArgs(args []string) (bool, error) {
	hasSlash := 0
	for _, s := range args {
		if strings.Contains(s, "/") {
			hasSlash++
		}
	}
	switch {
	case hasSlash > 0 && hasSlash == len(args):
		return true, nil
	case hasSlash > 0 && hasSlash != len(args):
		baseCmd := "cmd"
		if len(os.Args) > 0 {
			baseCmdSlice := strings.Split(os.Args[0], "/")
			baseCmd = baseCmdSlice[len(baseCmdSlice)-1]
		}
		return true, fmt.Errorf("there is no need to specify a resource type as a separate argument when passing arguments in resource/name form (e.g. '%s get resource/<resource_name>' instead of '%s get resource resource/<resource_name>'", baseCmd, baseCmd)
	default:
		return false, nil
	}
}

// splitTypeName handles type/name resource formats and returns a resource tuple
// (empty or not), whether it successfully found one, and an error
func splitTypeName(s string) (typeNames, bool, error) {
	if !strings.Contains(s, "/") {
		return typeNames{}, false, nil
	}
	seg := strings.Split(s, "/")
	if len(seg) != 2 {
		return typeNames{}, false, fmt.Errorf("arguments in resource/name form may not have more than one slash")
	}
	res, name := seg[0], seg[1]
	if len(res) == 0 || len(name) == 0 || len(resource.SplitResourceArgument(res)) != 1 {
		return typeNames{}, false, fmt.Errorf("arguments in resource/name form must have a single resource and name")
	}
	if strings.Contains(name, ",") {
		return typeNames{}, false, fmt.Errorf("arguments in resource/name form may not have commas in the name")
	}
	return typeNames{Resource: res, Names: []string{name}}, true, nil
}

// Map translates a typeGroupNames into a kind or category. A reference with a
// group is considered a resource or kind. For native resources, we have to
// map resources to kind. For CRD resources we don't care because the cateogies
// table already has kind and resource.
func (tgn typeGroupNames) Map() (kind, category string) {
	if tgn.Group == "" {
		return "", tgn.Type
	}

	if !nativeScheme.IsGroupRegistered(tgn.Group) {
		// this is a CRD for which we register singular and plural categories
		return "", tgn.Type
	}

	kind, ok := nativeKinds[tgn.Type]
	if !ok {
		// nothing to map. We don't know this type, or it is already a kind.
		return tgn.Type, ""
	}

	return kind, ""
}
