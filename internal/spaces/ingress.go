// Copyright 2024 Upbound Inc
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

package spaces

import (
	"context"
	"crypto/x509"
	"encoding/pem"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/internal/version"
)

func GetIngressFromSpace(ctx context.Context, space v1alpha1.Space, bearer string) (host string, ca []byte, err error) {
	if space.Status.APIURL == "" {
		return "", []byte{}, errors.New("API URL not defined on space")
	}

	cfg := &rest.Config{
		Host:        space.Status.APIURL,
		APIPath:     "/apis",
		UserAgent:   version.UserAgent(),
		BearerToken: bearer,
	}

	connectClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return "", []byte{}, errors.New("unable to construct connect client to space")
	}

	var ingressPublic corev1.ConfigMap
	if err := connectClient.Get(ctx, types.NamespacedName{Namespace: "upbound-system", Name: "ingress-public"}, &ingressPublic); err != nil {
		return "", []byte{}, errors.Wrap(err, "failed to query public ingress configmap")
	}

	var ok bool
	if host, ok = ingressPublic.Data["ingress-host"]; !ok {
		return "", []byte{}, errors.Wrap(err, `"ingress-host" not found in public ingress configmap`)
	}
	if caString, ok := ingressPublic.Data["ingress-ca"]; !ok {
		return "", []byte{}, errors.Wrap(err, `"ingress-ca" not found in public ingress configmap`)
	} else if err = ensureCertificateAuthorityData(caString); err != nil {
		return "", []byte{}, err
	} else {
		ca = []byte(caString)
	}

	return host, ca, err
}

func ensureCertificateAuthorityData(tlsCert string) error {
	block, _ := pem.Decode([]byte(tlsCert))
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.New("CA string does not contain PEM certificate data")
	}

	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "CA cannot be parsed to x509 certificate")
	}
	return nil
}
