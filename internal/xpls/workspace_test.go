package xpls

import (
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var testComposition = []byte(`
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: vpcpostgresqlinstances.aws.database.example.org
  labels:
    uxp-guide: getting-started
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - name: vpc
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: VPC
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.0.0/16
            enableDnsSupport: true
            enableDnsHostNames: true
    - name: subnet-a
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-east-1a
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.64.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-east-1a
`)

var multipleObject = `
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.database.example.org
---
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: some.other.xrd
`

func TestParse(t *testing.T) {
	cases := map[string]struct {
		reason string
		ws     *Workspace
		nodes  map[NodeIdentifier]struct{}
		err    error
	}{
		"ErrorRootNotExist": {
			reason: "Should return an error if the workspace root does not exist.",
			ws:     NewWorkspace("/ws", WithFS(afero.NewMemMapFs())),
			err:    &os.PathError{Op: "open", Path: "/ws", Err: afero.ErrFileNotFound},
		},
		"SuccessfulEmpty": {
			reason: "Should not return an error if the workspace is empty.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				return fs
			}())),
		},
		"SuccessfulNoKubernetesObjects": {
			// NOTE(hasheddan): this test reflects current behavior, but we
			// should be surfacing errors / diagnostics if we fail to parse
			// objects in a package unless they are specified to be ignored. We
			// likely also want skip any non-YAML files by default as we do in
			// package parsing.
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}())),
		},
		"SuccessfulParseComposition": {
			reason: "Should add a package node for Composition and every embedded resource.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/composition.yaml", testComposition, os.ModePerm)
				return fs
			}())),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "VPC")):               {},
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "Subnet")):            {},
				nodeID("vpcpostgresqlinstances.aws.database.example.org", xpextv1.CompositionGroupVersionKind): {},
			},
		},
		"SuccessfulParseMultipleSameFile": {
			reason: "Should add a package node for every resource when multiple objects exist in single file.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/multiple.yaml", []byte(multipleObject), os.ModePerm)
				return fs
			}())),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("compositepostgresqlinstances.database.example.org", xpextv1.CompositeResourceDefinitionGroupVersionKind): {},
				nodeID("some.other.xrd", xpextv1.CompositeResourceDefinitionGroupVersionKind):                                    {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.ws.Parse(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.nodes) != len(tc.ws.nodes) {
				t.Errorf("\n%s\nParse(...): -want node count: %d, +got node count: %d", tc.reason, len(tc.nodes), len(tc.ws.nodes))
			}
			for id := range tc.ws.nodes {
				if _, ok := tc.nodes[id]; !ok {
					t.Errorf("\n%s\nParse(...): missing node:\n%v", tc.reason, id)
				}
			}
		})
	}
}
