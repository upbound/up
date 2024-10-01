// Copyright 2024 Upbound Inc.
// All rights reserved

package resource

import (
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	queryv1alpha2 "github.com/upbound/up-sdk-go/apis/query/v1alpha2"
)

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: queryv1alpha2.SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
