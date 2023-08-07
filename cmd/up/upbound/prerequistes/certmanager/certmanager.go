package certmanager

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

var (
	chartName     = "cert-manager"
	certMgrURL, _ = url.Parse("https://charts.jetstack.io")

	// Chart version to be installed
	version = "v1.11.0"
	// Ensure CRDs are installed for the chart.
	values = map[string]any{
		"installCRDs": "true",
	}

	certificatesCRD = "certificates.cert-manager.io"

	errCreateNamespace = "failed to create namespace"

	errFmtCreateHelmManager = "failed to create helm manager for %s"
	errFmtCreateK8sClient   = "failed to create kubernetes client for helm chart %s"
)

// CertManager represents a Helm manager
type CertManager struct {
	mgr       install.Manager
	crdclient *apixv1client.ApiextensionsV1Client
	kclient   kubernetes.Interface
}

// New constructs a new CertManager instance that can used to install the
// cert-manager chart.
func New(config *rest.Config) (*CertManager, error) {
	mgr, err := helm.NewManager(config,
		chartName,
		certMgrURL,
		helm.WithNamespace(chartName),
	)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateHelmManager, chartName))
	}
	crdclient, err := apixv1client.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}

	return &CertManager{
		mgr:       mgr,
		crdclient: crdclient,
		kclient:   kclient,
	}, nil
}

// GetName returns the name of the cert-manager chart.
func (c *CertManager) GetName() string {
	return chartName
}

// Install performs a Helm install of the chart.
func (c *CertManager) Install() error {
	if c.IsInstalled() {
		// nothing to do
		return nil
	}

	// create namespace before creating chart.
	_, err := c.kclient.CoreV1().
		Namespaces().
		Create(context.Background(),
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: chartName,
				},
			}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, errCreateNamespace)
	}

	return c.mgr.Install(version, values)
}

// IsInstalled checks if cert-managed has been installed in the target cluster.
func (c *CertManager) IsInstalled() bool {
	_, err := c.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			certificatesCRD,
			metav1.GetOptions{},
		)
	return !kerrors.IsNotFound(err)
}
