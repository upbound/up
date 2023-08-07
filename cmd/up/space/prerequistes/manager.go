package prerequistes

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/cmd/up/space/prerequistes/certmanager"
	"github.com/upbound/up/cmd/up/space/prerequistes/uxp"
)

var (
	errCreatePrerequiste = "failed to instantiate prerequiste manager"
)

// Prerequiste defines the API that is used to interogate an installation
// prerequiste.
type Prerequiste interface {
	GetName() string

	Install() error
	IsInstalled() bool
}

// Manager provides APIs for interacting with Prerequistes within the target
// cluster.
type Manager struct {
	prereqs []Prerequiste
}

// Status represents the the overall status of the Prerequistes within the
// target cluster.
type Status struct {
	NotInstalled []Prerequiste
}

// New constructs a new Manager for working with installation Prerequistes.
func New(config *rest.Config) (*Manager, error) {
	prereqs := []Prerequiste{}
	certmanager, err := certmanager.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequiste)
	}
	prereqs = append(prereqs, certmanager)

	uxp, err := uxp.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequiste)
	}
	prereqs = append(prereqs, uxp)

	return &Manager{
		prereqs: prereqs,
	}, nil
}

// Check performs IsInstalled checks for each of the Prerequistes against the
// target cluster.
func (m *Manager) Check() *Status {
	notInstalled := []Prerequiste{}
	for _, p := range m.prereqs {
		if !p.IsInstalled() {
			notInstalled = append(notInstalled, p)
		}
	}

	return &Status{
		NotInstalled: notInstalled,
	}
}
