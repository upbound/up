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

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/printers"
	cliresource "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go/apis/common"
	queryv1alpha1 "github.com/upbound/up-sdk-go/apis/query/v1alpha1"
	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/upbound"
)

// afterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *cmd) afterApply() error {
	if c.ShowLabels && c.OutputFormat != "" && c.OutputFormat != "wide" {
		return fmt.Errorf("--show-labels option cannot be used with %s printer", c.OutputFormat)
	}

	c.printFlags = get.NewGetPrintFlags()
	c.printFlags.NoHeaders = &c.NoHeaders
	c.printFlags.OutputFormat = ptr.To(strings.TrimPrefix(c.OutputFormat, "="))

	c.printFlags.CustomColumnsFlags.NoHeaders = c.NoHeaders

	c.printFlags.HumanReadableFlags.ColumnLabels = &c.ColumnLabels
	c.printFlags.HumanReadableFlags.ShowKind = &c.ShowKind
	c.printFlags.HumanReadableFlags.ShowLabels = &c.ShowLabels
	c.printFlags.HumanReadableFlags.SortBy = &c.SortBy

	c.printFlags.TemplateFlags.TemplateArgument = &c.Template
	c.printFlags.TemplateFlags.AllowMissingKeys = &c.AllowMissingKeys

	c.printFlags.JSONYamlPrintFlags.ShowManagedFields = c.ShowManagedFields

	return nil
}

func (c *cmd) Run(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, queryTemplate resource.QueryObject, kubeconfig *rest.Config, notFound NotFound) error { // nolint:gocyclo // mostly taken from kubectl get. We don't want to divert.
	tgns, errs := ParseTypesAndNames(c.Resources...)
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	gkNames, categoryNames := SplitGroupKindAndCategories(tgns)

	if upCtx.WrapTransport != nil {
		kubeconfig.Wrap(upCtx.WrapTransport)
	}

	// pre-check that the server supports the v1alpha1 query API
	if err := checkQueryAPIAvailability(kubeconfig); err != nil {
		return err
	}

	kc, err := client.New(kubeconfig, client.Options{Scheme: queryScheme})
	if err != nil {
		return errors.Wrap(err, "failed to create client")
	}

	// create queries
	var querySpecs []*queryv1alpha1.QuerySpec
	for gk, names := range gkNames {
		if len(names) == 0 {
			query := createQuerySpec(types.NamespacedName{Namespace: c.namespace}, gk, nil, c.OutputFormat, c.Template)
			querySpecs = append(querySpecs, query)
			continue
		}
		for _, name := range names {
			query := createQuerySpec(types.NamespacedName{Namespace: c.namespace, Name: name}, gk, nil, c.OutputFormat, c.Template)
			querySpecs = append(querySpecs, query)
		}
	}
	for cat, names := range categoryNames {
		catList := []string{cat}
		if cat == AllCategory {
			catList = nil
		}
		if len(names) == 0 {
			query := createQuerySpec(types.NamespacedName{Namespace: c.namespace}, metav1.GroupKind{}, catList, c.OutputFormat, c.Template)
			querySpecs = append(querySpecs, query)
			continue
		}
		for _, name := range names {
			query := createQuerySpec(types.NamespacedName{Namespace: c.namespace, Name: name}, metav1.GroupKind{}, catList, c.OutputFormat, c.Template)
			querySpecs = append(querySpecs, query)
		}
	}

	// send queries and collect objects
	var infos []*cliresource.Info
	gks := sets.New[runtimeschema.GroupKind]()
	for qi, spec := range querySpecs {
		var cursor string
		var page int
		for {
			spec := spec.DeepCopy()
			spec.QueryTopLevelResources.QueryResources.Page.Cursor = cursor
			query := queryTemplate.DeepCopyQueryObject().SetSpec(spec)

			// print query for debugging
			if c.Flags.Debug > 0 {
				kinds, _, err := queryScheme.ObjectKinds(query)
				if err != nil {
					return errors.Wrap(err, "failed to get object kinds")
				}
				if len(kinds) != 1 {
					return errors.Errorf("expected exactly one kind, got %d", len(kinds))
				}
				query := query.DeepCopyQueryObject()
				query.GetObjectKind().SetGroupVersionKind(queryv1alpha1.SchemeGroupVersion.WithKind(kinds[0].Kind))
				bs, err := yaml.Marshal(query)
				if err != nil {
					return errors.Wrap(err, "failed to marshal query")
				}
				fmt.Fprintf(kongCtx.Stderr, "Sending query:\n\n%s\n", string(bs)) // nolint:errcheck // just debug output
			}

			// send request
			if err := kc.Create(ctx, query); err != nil {
				return errors.Wrap(err, "SpaceQuery request failed")
			}
			resp := query.GetResponse()
			for _, w := range resp.Warnings {
				pterm.Warning.Printfln("Warning: %s", w)
			}

			// collect objects
			if len(resp.Tables) > 0 {
				for i := range resp.Tables {
					tbl := &resp.Tables[i]
					t := &metav1.Table{
						TypeMeta: metav1.TypeMeta{
							APIVersion: metav1.SchemeGroupVersion.String(),
							Kind:       "Table",
						},
						ColumnDefinitions: tbl.Columns,
						Rows:              tbl.Rows,
					}
					for i := range tbl.Rows {
						r := &tbl.Rows[i]
						if len(r.Object.Raw) > 0 && r.Object.Object == nil {
							u := &unstructured.Unstructured{}
							r.Object.Object = u
							if err := json.Unmarshal(r.Object.Raw, &u.Object); err != nil {
								return fmt.Errorf("failed to unmarshal object: %w", err)
							}
						}
					}
					info := &cliresource.Info{
						Client: nil,
						Mapping: &meta.RESTMapping{
							Resource: runtimeschema.GroupVersionResource{
								Group:   tbl.GroupVersionKind.Group,
								Version: tbl.GroupVersionKind.Version,
								// used to switch to another printer. We don't know
								// the resource anyway and it has no impact on the
								// output other than creation of a new printer.
								Resource: fmt.Sprintf("table-%d", qi+i),
							},
							GroupVersionKind: runtimeschema.GroupVersionKind{
								Group:   tbl.GroupVersionKind.Group,
								Version: tbl.GroupVersionKind.Version,
								Kind:    tbl.GroupVersionKind.Kind,
							},
							Scope: RESTScopeNameFunc(meta.RESTScopeNameRoot),
						},
						Object: t,
					}
					if hasNamespacedResources(tbl) {
						info.Mapping.Scope = RESTScopeNameFunc(meta.RESTScopeNameNamespace)
					} else {
						// remove namespace column
						for i, col := range t.ColumnDefinitions {
							if col.Name == "Namespace" {
								t.ColumnDefinitions = append(t.ColumnDefinitions[:i], t.ColumnDefinitions[i+1:]...)
								for j := range tbl.Rows {
									t.Rows[j].Cells = append(t.Rows[j].Cells[:i], t.Rows[j].Cells[i+1:]...)
								}
								break
							}
						}
					}
					infos = append(infos, info)
					gks.Insert(runtimeschema.GroupKind{Group: tbl.GroupVersionKind.Group, Kind: tbl.GroupVersionKind.Kind})
				}
			} else {
				for _, obj := range resp.Objects {
					if obj.Object == nil {
						return fmt.Errorf("received unexpected nil object in response")
					}

					u := &unstructured.Unstructured{Object: obj.Object.Object}
					infos = append(infos, &cliresource.Info{
						Client: nil,
						Mapping: &meta.RESTMapping{
							Resource: runtimeschema.GroupVersionResource{
								Group: u.GroupVersionKind().Group,
							},
							GroupVersionKind: u.GroupVersionKind(),
							Scope:            RESTScopeNameFunc(u.GetNamespace()),
						},
						Namespace:       u.GetNamespace(),
						Name:            u.GetName(),
						Source:          types.NamespacedName{Namespace: obj.ControlPlane.Namespace, Name: obj.ControlPlane.Name}.String(),
						Object:          u,
						ResourceVersion: u.GetResourceVersion(),
					})
					gks.Insert(u.GroupVersionKind().GroupKind())
				}
			}

			// do paging
			cursor = resp.Cursor.Next
			page++
			if cursor == "" {
				break
			}
			if c.Flags.Debug > 0 {
				fmt.Fprintf(kongCtx.Stderr, "Fetching page %d\n", page) // nolint:errcheck // just debug output
			}
		}
	}

	// print objects
	showKind := c.ShowKind || gks.Len() > 1 || len(categoryNames)+len(gkNames) > 1
	humanReadableOutput := (c.OutputFormat == "" && c.Template == "") || c.OutputFormat == "wide"
	if humanReadableOutput {
		return c.humanReadablePrintObjects(kongCtx, infos, showKind, notFound)
	}
	return c.printGeneric(kongCtx, infos)
}

func (c *cmd) humanReadablePrintObjects(kongCtx *kong.Context, infos []*cliresource.Info, printWithKind bool, notFound NotFound) error { // nolint:gocyclo // mostly taken from kubectl get. We don't want to divert.
	objs := make([]kruntime.Object, len(infos))
	for i, info := range infos {
		objs[i] = info.Object
	}

	var positioner get.OriginalPositioner
	if len(c.SortBy) > 0 {
		robjs := make([]kruntime.Object, len(objs))
		copy(robjs, objs)
		sorter := get.NewRuntimeSorter(robjs, c.SortBy)
		if err := sorter.Sort(); err != nil {
			return err
		}
		positioner = sorter
	}

	trackingWriter := &trackingWriterWrapper{Delegate: kongCtx.Stdout}   // track if we write any output
	separatorWriter := &separatorWriterWrapper{Delegate: trackingWriter} // output an empty line separating output

	w := printers.GetNewTabWriter(separatorWriter)

	var (
		allErrs     []error
		errs        = sets.NewString()
		printer     printers.ResourcePrinter
		lastMapping *meta.RESTMapping
	)
	for ix := range objs {
		var mapping *meta.RESTMapping
		var info *cliresource.Info
		if positioner != nil {
			info = infos[positioner.OriginalPosition(ix)]
			mapping = info.Mapping
		} else {
			info = infos[ix]
			mapping = info.Mapping
		}

		if shouldGetNewPrinterForMapping(printer, lastMapping, mapping) {
			if err := w.Flush(); err != nil {
				return err
			}
			w.SetRememberedWidths(nil)

			// add linebreaks between resource groups (if there is more than one)
			// when it satisfies all following 3 conditions:
			// 1) it's not the first resource group
			// 2) it has row header
			// 3) we've written output since the last time we started a new set of headers
			if lastMapping != nil && !c.NoHeaders && trackingWriter.Written > 0 {
				separatorWriter.SetReady(true)
			}

			var err error
			printer, err = c.createPrinter(mapping, false, printWithKind)
			if err != nil {
				if !errs.Has(err.Error()) {
					errs.Insert(err.Error())
					allErrs = append(allErrs, err)
				}
				continue
			}

			lastMapping = mapping
		}

		if err := printer.PrintObj(info.Object, w); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if trackingWriter.Written == 0 && len(allErrs) == 0 {
		if err := notFound.PrintMessage(); err != nil {
			return err
		}
	}
	return kerrors.NewAggregate(allErrs)
}

func (c *cmd) printGeneric(kongCtx *kong.Context, infos []*cliresource.Info) error { // nolint:gocyclo // mostly taken from kubectl get. We don't want to divert.
	// we flattened the data from the builder, so we have individual items, but now we'd like to either:
	// 1. if there is more than one item, combine them all into a single list
	// 2. if there is a single item and that item is a list, leave it as its specific list
	// 3. if there is a single item and it is not a list, leave it as a single item
	printer, err := c.createPrinter(nil, false, false)
	if err != nil {
		return err
	}

	var obj kruntime.Object
	var errs []error
	if len(infos) != 1 {
		// we have zero or multple items, so coerce all items into a list.
		// we don't want an *unstructured.Unstructured list yet, as we
		// may be dealing with non-unstructured objects. Compose all items
		// into an corev1.List, and then decode using an unstructured scheme.
		list := corev1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       "List",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{},
		}
		for _, info := range infos {
			list.Items = append(list.Items, kruntime.RawExtension{Object: info.Object})
		}

		listData, err := json.Marshal(list)
		if err != nil {
			return err
		}

		converted, err := kruntime.Decode(unstructured.UnstructuredJSONScheme, listData)
		if err != nil {
			return err
		}

		obj = converted
	} else {
		obj = infos[0].Object
	}

	isList := meta.IsListType(obj)
	if isList {
		items, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}

		// take the items and create a new list for display
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}
		if listMeta, err := meta.ListAccessor(obj); err == nil {
			list.Object["metadata"] = map[string]interface{}{
				"resourceVersion": listMeta.GetResourceVersion(),
			}
		}

		for _, item := range items {
			list.Items = append(list.Items, *item.(*unstructured.Unstructured))
		}
		if err := printer.PrintObj(list, kongCtx.Stdout); err != nil {
			errs = append(errs, err)
		}
		return kerrors.Reduce(kerrors.Flatten(kerrors.NewAggregate(errs)))
	}

	if printErr := printer.PrintObj(obj, kongCtx.Stdout); printErr != nil {
		errs = append(errs, printErr)
	}

	return kerrors.Reduce(kerrors.Flatten(kerrors.NewAggregate(errs)))
}

func (c *cmd) createPrinter(mapping *meta.RESTMapping, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) { // nolint:gocyclo // mostly taken from kubectl get. We don't want to divert.
	// make a new copy of current flags / opts before mutating
	printFlags := c.printFlags.Copy()

	if mapping != nil {
		printFlags.SetKind(mapping.GroupVersionKind.GroupKind())
	}
	if withNamespace {
		if err := printFlags.EnsureWithNamespace(); err != nil {
			return nil, err
		}
	}
	if withKind {
		if err := printFlags.EnsureWithKind(); err != nil {
			return nil, err
		}
	}

	printer, err := printFlags.ToPrinter()
	if err != nil {
		return nil, err
	}
	printer, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(printer, nil)
	if err != nil {
		return nil, err
	}

	if len(c.SortBy) > 0 {
		printer = &get.SortingPrinter{Delegate: printer, SortField: c.SortBy}
	}

	return printer.PrintObj, nil
}

func createQuerySpec(nname types.NamespacedName, gk metav1.GroupKind, categories []string, outputFormat string, tmpl string) *queryv1alpha1.QuerySpec {
	// retrieve minimal schema for the given output format
	var obj *common.JSON
	var tbl *queryv1alpha1.QueryTable
	switch outputFormat {
	case "", "wide":
		if tmpl == "" {
			tbl = &queryv1alpha1.QueryTable{}
		} else {
			obj = &common.JSON{Object: true} // everything
		}
	case "name":
		obj = &common.JSON{Object: map[string]interface{}{
			"kind":       true,
			"apiVersion": true,
			"metadata": map[string]interface{}{
				"name":      true,
				"namespace": true,
			},
		}}
	default:
		// TODO: would be good to have a way to exclude managed fields
		obj = &common.JSON{Object: true} // everything
	}

	return &queryv1alpha1.QuerySpec{
		QueryTopLevelResources: queryv1alpha1.QueryTopLevelResources{
			Filter: queryv1alpha1.QueryTopLevelFilter{
				QueryFilter: queryv1alpha1.QueryFilter{
					Namespace:  nname.Namespace,
					Name:       nname.Name,
					Categories: categories,
					Group:      gk.Group,
					Kind:       gk.Kind,
				},
			},
			QueryResources: queryv1alpha1.QueryResources{
				Limit:  500,
				Cursor: true,
				Objects: &queryv1alpha1.QueryObjects{
					ControlPlane: true,
					Object:       obj,
					Table:        tbl,
				},
			},
		},
	}
}

func shouldGetNewPrinterForMapping(printer printers.ResourcePrinter, lastMapping, mapping *meta.RESTMapping) bool {
	return printer == nil || lastMapping == nil || mapping == nil || mapping.Resource != lastMapping.Resource
}

type RESTScopeNameFunc string

func (f RESTScopeNameFunc) Name() meta.RESTScopeName {
	if f == "" {
		return meta.RESTScopeNameRoot
	}
	return meta.RESTScopeNameNamespace
}

// hasNamespacedResources looks for cells with a non-empty namespace.
func hasNamespacedResources(t *queryv1alpha1.QueryResponseTable) bool {
	nsColumn := -1
	for i, col := range t.Columns {
		if col.Name == "Namespace" {
			nsColumn = i
			break
		}
	}

	if nsColumn == -1 {
		return false
	}

	for _, row := range t.Rows {
		if nsColumn >= len(row.Cells) {
			continue
		}
		if row.Cells[nsColumn] == "" || row.Cells[nsColumn] == nil {
			continue
		}
		return true
	}

	return false
}

func checkQueryAPIAvailability(kubeconfig *rest.Config) error { //nolint:gocyclo // it is what it is ¯\_(ツ)_/¯
	cl, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client: %v")
	}

	// check for query API group
	groups, err := cl.Discovery().ServerGroups()
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to discover server groups")
	}
	foundV1alpha1 := false
	foundLaterVersion := false
	for _, g := range groups.Groups {
		if g.Name != queryv1alpha1.Group {
			continue
		}
		for _, v := range g.Versions {
			if v.Version == queryv1alpha1.Version {
				foundV1alpha1 = true
			} else {
				foundLaterVersion = true
			}
		}
	}

	if !foundV1alpha1 && foundLaterVersion {
		return errors.Errorf("server does not support the %s/%s API anymore. Update 'up' to a later version.", queryv1alpha1.Group, queryv1alpha1.Version)
	}
	if !foundV1alpha1 && !foundLaterVersion {
		return errors.Errorf("server does not support the %s/%s API. Make sure the 'apollo' tech preview feature is enabled on the Space.", queryv1alpha1.Group, queryv1alpha1.Version)
	}

	return nil
}
