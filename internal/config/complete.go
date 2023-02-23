package config

import (
	"github.com/upbound/up-sdk-go/service/organizations"
)

type CompleteHelpers struct {
	OrgsClient *organizations.Client
}

var Helpers CompleteHelpers
