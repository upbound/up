package migration

import (
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Context includes common data that migration commands may utilize.
type Context struct {
	KubeCore    corev1.CoreV1Interface
	KubeDynamic dynamic.Interface
	Namespace   string
}
