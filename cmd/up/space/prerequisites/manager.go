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

package prerequisites

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/cmd/up/space/defaults"
	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites/certmanager"
	"github.com/upbound/up/cmd/up/space/prerequisites/ingressnginx"
	"github.com/upbound/up/cmd/up/space/prerequisites/opentelemetrycollector"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/helm"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/kubernetes"
	"github.com/upbound/up/cmd/up/space/prerequisites/uxp"
)

var (
	errCreatePrerequisite = "failed to instantiate prerequisite manager"
)

// Prerequisite defines the API that is used to interogate an installation
// prerequisite.
type Prerequisite interface {
	GetName() string

	Install() error
	IsInstalled() bool
}

// Manager provides APIs for interacting with Prerequisites within the target
// cluster.
type Manager struct {
	prereqs []Prerequisite
}

// Status represents the the overall status of the Prerequisite within the
// target cluster.
type Status struct {
	NotInstalled []Prerequisite
}

// New constructs a new Manager for working with installation Prerequisites.
func New(config *rest.Config, defs *defaults.CloudConfig, features *feature.Flags) (*Manager, error) {
	prereqs := []Prerequisite{}
	certmanager, err := certmanager.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, certmanager)

	uxp, err := uxp.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, uxp)

	svcType := ingressnginx.NodePort
	if defs != nil && defs.PublicIngress {
		svcType = ingressnginx.LoadBalancer
	}

	ingress, err := ingressnginx.New(config, svcType)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, ingress)

	pk8s, err := kubernetes.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, pk8s)

	phelm, err := helm.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, phelm)

	if features.Enabled(spacefeature.EnableAlphaSharedTelemetry) {
		otelopr, err := opentelemetrycollector.New(config)
		if err != nil {
			return nil, errors.Wrap(err, errCreatePrerequisite)
		}
		prereqs = append(prereqs, otelopr)
	}

	return &Manager{
		prereqs: prereqs,
	}, nil
}

// Check performs IsInstalled checks for each of the Prerequisites against the
// target cluster.
func (m *Manager) Check() *Status {
	notInstalled := []Prerequisite{}
	for _, p := range m.prereqs {
		if !p.IsInstalled() {
			notInstalled = append(notInstalled, p)
		}
	}

	return &Status{
		NotInstalled: notInstalled,
	}
}
