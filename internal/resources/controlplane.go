package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

var (
	// ControlPlaneGVK is the GroupVersionKind used for
	// provider-kubernetes ProviderConfig.
	ControlPlaneGVK = schema.GroupVersionKind{
		Group:   "spaces.upbound.io",
		Version: "v1beta1",
		Kind:    "ControlPlane",
	}
	ControlPlaneGVR = schema.GroupVersionResource{
		Group:    "spaces.upbound.io",
		Version:  "v1beta1",
		Resource: "controlplanes",
	}
)

// ControlPlane represents the ControlPlane CustomResource and extends an
// unstructured.Unstructured.
type ControlPlane struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *ControlPlane) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (c *ControlPlane) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// GetControlPlaneID returns the MXP ID associated with the ControlPlane.
func (c *ControlPlane) GetControlPlaneID() string {
	id, err := fieldpath.Pave(c.Object).GetString("status.controlPlaneID")
	if err != nil {
		return ""
	}
	return id
}

// SetWriteConnectionSecretToReference of this composite resource claim.
func (c *ControlPlane) SetWriteConnectionSecretToReference(ref *xpv1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

func (c *ControlPlane) GetConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out)
	return out
}
