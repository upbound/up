/*
Copyright 2021 Upbound Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package uxp

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/uxp"
)

const (
	defaultSecretKey = "token"
)

// AfterApply sets default values in command before assignment and validation.
func (c *connectCmd) AfterApply(uxpCtx *uxp.Context) error {
	client, err := kubernetes.NewForConfig(uxpCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	c.stdin = os.Stdin
	return nil
}

// connectCmd connects UXP to Upbound Cloud.
type connectCmd struct {
	kClient kubernetes.Interface
	stdin   io.Reader

	CPToken string `arg:"" required:"" help:"Token used to connect self-hosted control plane."`

	TokenSecretName string `default:"upbound-control-plane-token" help:"Name of secret that will be populated with token data."`
}

// Run executes the connect command.
func (c *connectCmd) Run(kong *kong.Context, uxpCtx *uxp.Context) error {
	// TODO(hasheddan): consider implementing a custom decoder
	if c.CPToken == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.CPToken = string(b)
	}
	// Remove any trailing newlines from token, which can make piping output
	// from other commands more convenient.
	c.CPToken = strings.TrimSpace(c.CPToken)

	// Create namespace if it does not exist.
	_, err := c.kClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: uxpCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.TokenSecretName,
		},
		StringData: map[string]string{
			defaultSecretKey: c.CPToken,
		},
	}
	_, err = c.kClient.CoreV1().Secrets(uxpCtx.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil && kerrors.IsAlreadyExists(err) {
		_, err = c.kClient.CoreV1().Secrets(uxpCtx.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	}
	return err
}
