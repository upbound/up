package object

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1ext "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestValidatorsFroObj(t *testing.T) {
	type args struct {
		o runtime.Object
	}

	type want struct {
		err string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InvalidSchemaForXRD": {
			reason: "Should err due to invalid schema definition",
			args: args{
				o: &xpv1ext.CompositeResourceDefinition{
					ObjectMeta: v1.ObjectMeta{
						Name: "xpostgresqlinstances.database.example.org",
					},
					Spec: xpv1ext.CompositeResourceDefinitionSpec{
						Group: "database.example.org",
						Names: extv1.CustomResourceDefinitionNames{
							Kind:   "XPostgreSQLInstance",
							Plural: "xpostgresqlinstances",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:   "PostgreSQLInstance",
							Plural: "postgresqlinstances",
						},
						ConnectionSecretKeys: []string{
							"username",
							"password",
							"endpoint",
							"port",
						},
						Versions: []xpv1ext.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Served:        true,
								Referenceable: true,
								Schema: &xpv1ext.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte("{\"properties\":{\"parameters\":{\"storageGB\":{\"type\":\"integer\"}},\"required\":[\"storageGB\"]},\"required\":[\"parameters\"],\"type\":\"object\"}"),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: "json: cannot unmarshal array into Go struct field JSONSchemaProps.properties of type v1.JSONSchemaProps",
			},
		},
		"ValidSchemaForXRD": {
			reason: "Valid schema should not error",
			args: args{
				o: &xpv1ext.CompositeResourceDefinition{
					ObjectMeta: v1.ObjectMeta{
						Name: "xpostgresqlinstances.database.example.org",
					},
					Spec: xpv1ext.CompositeResourceDefinitionSpec{
						Group: "database.example.org",
						Names: extv1.CustomResourceDefinitionNames{
							Kind:   "XPostgreSQLInstance",
							Plural: "xpostgresqlinstances",
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:   "PostgreSQLInstance",
							Plural: "postgresqlinstances",
						},
						ConnectionSecretKeys: []string{
							"username",
							"password",
							"endpoint",
							"port",
						},
						Versions: []xpv1ext.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Served:        true,
								Referenceable: true,
								Schema: &xpv1ext.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte("{\"properties\":{\"spec\":{\"properties\":{\"parameters\":{\"properties\":{\"storageGB\":{\"type\":\"integer\"}},\"required\":[\"storageGB\"],\"type\":\"object\"}},\"required\":[\"parameters\"],\"type\":\"object\"}},\"type\":\"object\"}"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ValidatorsForObj(tc.args.o)

			if err != nil {
				if diff := cmp.Diff(tc.want.err, err.Error()); diff != "" {
					t.Errorf("\n%s\nValidatorsFroObj(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
