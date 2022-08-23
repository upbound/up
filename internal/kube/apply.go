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
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/cmd/create"
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

// ImagePullApplicator constructs and creates or updates an image pull Secret.
type ImagePullApplicator struct {
	secret *SecretApplicator
}

// NewImagePullApplicator constructs a new ImagePullApplicator with the passed
// SecretApplicator.
// TODO(hasheddan): consider moving applicators to a common interface.
func NewImagePullApplicator(secret *SecretApplicator) *ImagePullApplicator {
	return &ImagePullApplicator{
		secret: secret,
	}
}

// Apply constructs an DockerConfig image pull Secret with the provided registry
// and credentials.
func (i *ImagePullApplicator) Apply(ctx context.Context, name, ns, user, pass, registry string) error {
	regAuth := &create.DockerConfigJSON{
		Auths: map[string]create.DockerConfigEntry{
			registry: {
				Username: user,
				Password: pass,
				Auth:     encodeDockerConfigFieldAuth(user, pass),
			},
		},
	}
	regAuthJSON, err := json.Marshal(regAuth)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: regAuthJSON,
		},
	}
	// Create image pull secret if it does not exist.
	return i.secret.Apply(ctx, ns, secret)
}

// encodeDockerConfigFieldAuth returns base64 encoding of the username and
// password string
// NOTE(hasheddan): this function comes directly from kubectl
// https://github.com/kubernetes/kubectl/blob/0f88fc6b598b7e883a391a477215afb080ec7733/pkg/cmd/create/create_secret_docker.go#L305
func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
