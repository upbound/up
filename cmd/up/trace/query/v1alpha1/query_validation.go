// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func (c *SpaceQuery) ValidateCreate() (errs field.ErrorList) {
	errs = append(errs, validation.ValidateObjectMeta(&c.ObjectMeta, false, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	if c.Spec == nil {
		errs = append(errs, field.Required(field.NewPath("spec"), "must be specified"))
	} else {
		errs = append(errs, c.Spec.validateCreate(field.NewPath("spec"))...)
	}
	if c.Response != nil {
		bs, _ := json.Marshal(c.Response) // nolint:errcheck,errchkjson
		errs = append(errs, field.Invalid(field.NewPath("response"), string(bs), "must not be specified"))
	}

	return errs
}

func (c *GroupQuery) ValidateCreate() (errs field.ErrorList) {
	errs = append(errs, validation.ValidateObjectMeta(&c.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	if c.Spec == nil {
		errs = append(errs, field.Required(field.NewPath("spec"), "must be specified"))
	} else {
		if c.Spec.Filter.ControlPlane.Namespace != c.Namespace {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("filter").Child("controlPlane").Child("namespace"), c.Spec.Filter.ControlPlane.Namespace, "must match .metadata.namespace"))
		}
		errs = append(errs, c.Spec.validateCreate(field.NewPath("spec"))...)
	}
	if c.Response != nil {
		bs, _ := json.Marshal(c.Response) // nolint:errcheck,errchkjson
		errs = append(errs, field.Invalid(field.NewPath("response"), string(bs), "must not be specified"))
	}

	return errs
}

func (c *Query) ValidateCreate() (errs field.ErrorList) {
	errs = append(errs, validation.ValidateObjectMeta(&c.ObjectMeta, true, validation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	if c.Spec == nil {
		errs = append(errs, field.Required(field.NewPath("spec"), "must be specified"))
	} else {
		if c.Spec.Filter.ControlPlane.Namespace != c.Namespace {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("filter").Child("controlPlane").Child("namespace"), c.Spec.Filter.ControlPlane.Namespace, "must match .metadata.namespace"))
		}
		if c.Spec.Filter.ControlPlane.Name != c.Name {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("filter").Child("controlPlane").Child("name"), c.Spec.Filter.ControlPlane.Name, "must match .metadata.name"))
		}

		errs = append(errs, c.Spec.validateCreate(field.NewPath("spec"))...)
	}
	if c.Response != nil {
		bs, _ := json.Marshal(c.Response) // nolint:errcheck,errchkjson
		errs = append(errs, field.Invalid(field.NewPath("response"), string(bs), "must not be specified"))
	}

	return errs
}

func (c *QuerySpec) validateCreate(pth *field.Path) (errs field.ErrorList) {
	errs = append(errs, c.Filter.ControlPlane.validateCreate(pth.Child("filter"))...)

	singleObject := len(c.Filter.IDs) == 1
	errs = append(errs, c.QueryResources.validateCreate(pth, singleObject)...)

	return errs
}

func (c *QueryFilterControlPlane) validateCreate(pth *field.Path) (errs field.ErrorList) { // nolint:unparam
	if c.Name != "" && c.Namespace == "" {
		errs = append(errs, field.Required(pth.Child("name"), "must specify a namespace if specifying a name"))
	}

	return errs
}

func (o *QueryOrder) validateCreate(pth *field.Path) (errs field.ErrorList) { // nolint:unparam
	n := 0
	if o.CreationTimestamp != "" {
		n++
	}
	if o.Name != "" {
		n++
	}
	if o.Namespace != "" {
		n++
	}
	if o.Group != "" {
		n++
	}
	if o.Kind != "" {
		n++
	}
	if o.ControlPlane != "" {
		n++
	}
	if n != 1 {
		errs = append(errs, field.Invalid(pth, o, "must specify exactly one of creationTimestamp, name, namespace, group, kind, cluster"))
	}

	return errs
}

func (c *QueryResources) validateCreate(pth *field.Path, singleObject bool) (errs field.ErrorList) { // nolint:gocyclo
	if c.Objects != nil {
		if c.Objects.Object != nil {
			if schema, ok := c.Objects.Object.Object.(bool); ok {
				if !schema {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("object"), c.Objects.Object.Object, "must be true"))
				}
			} else if _, ok := c.Objects.Object.Object.(map[string]interface{}); !ok {
				errs = append(errs, field.Invalid(pth.Child("objects").Child("object"), c.Objects.Object.Object, "must be true or an object"))
			}
		}

		for name, r := range c.Objects.Relations {
			if strings.HasSuffix(name, "+") {
				// check that name and name+ don't show up at the same time
				if _, ok := c.Objects.Relations[strings.TrimSuffix(name, "+")]; ok {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("relations"), name, fmt.Sprintf("cannot have both %q and %q relations", name, strings.TrimSuffix(name, "+"))))
				}

				// check that no direct descendant has a name or name+ relation
				if r.Objects != nil {
					for subName := range r.Objects.Relations {
						if subName == name || subName == strings.Trim(name, "+") {
							errs = append(errs, field.Invalid(pth.Child("objects").Child("relations").Key(name).Child("relations"), subName, fmt.Sprintf("cannot have a %q relation if the parent has a %q relation", subName, name)))
						}
					}
				}
			}

			if !singleObject {
				if r.Count {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("relations").Key(name).Child("count"), r.Count, "only valid in a relation when querying a single parent object"))
				}
				if r.Page.First > 0 {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("relations").Key(name).Child("page").Child("first"), r.Page.First, "only valid in a relation when querying a single parent object"))
				}
				if r.Page.Cursor != "" {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("relations").Key(name).Child("page").Child("cursor"), r.Page.Cursor, "only valid in a relation when querying a single parent object"))
				}
				if r.Cursor {
					errs = append(errs, field.Invalid(pth.Child("objects").Child("relations").Key(name).Child("cursor"), r.Cursor, "only valid in a relation when querying a single parent object"))
				}
			}

			errs = append(errs, r.validateCreate(pth.Child("objects").Child("relations").Key(name), false)...)
		}
	}

	for i, o := range c.Order {
		errs = append(errs, o.validateCreate(pth.Child("orders").Index(i))...)
	}

	if c.Limit <= 0 {
		errs = append(errs, field.Invalid(pth.Child("limit"), c.Limit, "must be greater than 0"))
	}
	if c.Page.First < 0 {
		errs = append(errs, field.Invalid(pth.Child("page").Child("first"), c.Page.First, "must be greater than or equal to 0"))
	}

	return errs
}
