package uxp

import (
	"k8s.io/client-go/rest"
)

// Context includes common data that UXP consumers may utilize.
type Context struct {
	Kubeconfig *rest.Config
	Namespace  string
}
