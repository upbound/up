package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// Upbound represents the Upbound CustomResource and extends an
// unstructured.Unstructured.
type Upbound struct {
	unstructured.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (s *Upbound) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(s.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}
