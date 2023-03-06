// Copyright 2023 Upbound Inc
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

package validator

import (
	"context"

	"k8s.io/kube-openapi/pkg/validation/validate"
)

type kubeValidator interface {
	Validate(data any) *validate.Result
}

// NewUsingContext returns a new validator that uses the provided kubeValidator
// with no context.
func NewUsingContext(k kubeValidator) *UsingContext {
	return &UsingContext{
		k: k,
	}
}

// UsingContext allows us to use kube-openapi validators without context usage
// to conform our interfaces that require it.
type UsingContext struct {
	k kubeValidator
}

// Validate calls the underlying kubeValidator's Validate method without a context.
func (uc *UsingContext) Validate(_ context.Context, data any) *validate.Result {
	return uc.k.Validate(data)
}
