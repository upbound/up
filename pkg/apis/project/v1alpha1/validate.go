// Copyright 2024 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"errors"
	"path/filepath"
)

func (p *Project) Validate() error {
	var errs []error

	if p.GetName() == "" {
		errs = append(errs, errors.New("name must not be empty"))
	}
	if p.Spec == nil {
		errs = append(errs, errors.New("spec must be present"))
	} else {
		errs = append(errs, p.Spec.Validate())
	}

	return errors.Join(errs...)
}

func (s *ProjectSpec) Validate() error {
	var errs []error

	if s.Repository == "" {
		errs = append(errs, errors.New("repository must not be empty"))
	}

	if s.Paths != nil {
		if s.Paths.APIs != "" && filepath.IsAbs(s.Paths.APIs) {
			errs = append(errs, errors.New("apis path must be relative"))
		}
		if s.Paths.Functions != "" && filepath.IsAbs(s.Paths.Functions) {
			errs = append(errs, errors.New("functions path must be relative"))
		}
		if s.Paths.Examples != "" && filepath.IsAbs(s.Paths.Examples) {
			errs = append(errs, errors.New("examples path must be relative"))
		}
	}

	return errors.Join(errs...)
}
