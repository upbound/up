// Copyright 2021 Upbound Inc
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

package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretApplicator creates or updates Secrets. In the event that the Secret
// exists and must be updated, it is completely replaced, not patched.
type SecretApplicator struct {
	kube kubernetes.Interface
}

// NewSecretApplicator constructs a SecretApplicator with the passed Kubernetes
// client.
func NewSecretApplicator(client kubernetes.Interface) *SecretApplicator {
	return &SecretApplicator{
		kube: client,
	}
}

// Apply creates or updates a Secret.
func (s *SecretApplicator) Apply(ctx context.Context, ns string, secret *corev1.Secret) error {
	_, err := s.kube.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && kerrors.IsAlreadyExists(err) {
		_, err = s.kube.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}
