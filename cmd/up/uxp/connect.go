package uxp

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kong"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/uxp"
)

const (
	defaultSecretName = "upbound-control-plane-token"
	defaultSecretKey  = "token"
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
	// Create namespace if it does not exist.
	_, err := c.kClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: uxpCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}
	_, err = c.kClient.CoreV1().Secrets(uxpCtx.Namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultSecretName,
		},
		StringData: map[string]string{
			defaultSecretKey: c.CPToken,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
