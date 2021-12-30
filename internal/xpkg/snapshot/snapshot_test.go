package snapshot

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/workspace"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

var (
	testSingleVersionCRD []byte
	testMultiVersionCRD  []byte
)

func init() {
	testSingleVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/single-version-crd.yaml")
	testMultiVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/multiple-version-crd.yaml")
}

func TestWSLoadValidators(t *testing.T) {
	cases := map[string]struct {
		reason     string
		opt        workspace.Option
		wsroot     string
		validators map[schema.GroupVersionKind]struct{}
		err        error
	}{
		"SuccessfulNoKubernetesObjects": {
			reason: "Should return an error if the path does not exist.",
			wsroot: "/ws",
			opt: workspace.WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/ws/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}()),
		},
		"SuccessfulLoadFromCRD": {
			reason: "Should add a validator for a CRD if it is valid.",
			wsroot: "/ws",
			opt: workspace.WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/ws/valid.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}()),
			validators: map[schema.GroupVersionKind]struct{}{
				schema.FromAPIVersionAndKind("acm.aws.crossplane.io/v1alpha1", "Certificate"): {},
			},
		},
		"SuccessfulLoadMultiVersionFromCRD": {
			reason: "Should add a validator for each version in a CRD if multiple are specified.",
			wsroot: "/ws",
			opt: workspace.WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/ws/multiversion.yaml", testMultiVersionCRD, os.ModePerm)
				return fs
			}()),
			validators: map[schema.GroupVersionKind]struct{}{
				xpextv1.CompositeResourceDefinitionGroupVersionKind:      {},
				xpextv1beta1.CompositeResourceDefinitionGroupVersionKind: {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := workspace.New(tc.wsroot, tc.opt)
			snap, err := New(
				tc.wsroot,
				logging.NewNopLogger(),
				WithWorkspace(ws),
				WithDepManager(NewMockDepManager()),
			)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadWSValidators(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.validators) != len(snap.validators) {
				t.Errorf("\n%s\nLoadWSValidators(...): -want validators count: %d, +got validators count: %d", tc.reason, len(tc.validators), len(snap.validators))
			}
			for id := range snap.validators {
				if _, ok := tc.validators[id]; !ok {
					t.Errorf("\n%s\nLoadWSValidators(...): missing validator:\n%v", tc.reason, id)
				}
			}
		})
	}
}

type MockDepManager struct{}

func NewMockDepManager() *MockDepManager { return &MockDepManager{} }

func (m *MockDepManager) View(context.Context, []v1beta1.Dependency) (*manager.View, error) {
	return nil, nil
}
func (m *MockDepManager) Versions(context.Context, v1beta1.Dependency) ([]string, error) {
	return nil, nil
}
