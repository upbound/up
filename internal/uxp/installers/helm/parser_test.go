package helm

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	// errBoom := errors.New("boom")
	cases := map[string]struct {
		reason string
		parser *Parser
		params map[string]interface{}
		err    error
	}{
		"SuccessfulBaseNoOverrides": {
			reason: "If no overrides are provided the base should be returned.",
			parser: &Parser{
				values: map[string]interface{}{
					"test": "value",
				},
			},
			params: map[string]interface{}{
				"test": "value",
			},
		},
		"SuccessfulBaseWithOverrides": {
			reason: "If base and overrides are provided then overrides should take precedence.",
			parser: &Parser{
				values: map[string]interface{}{
					"test": "value",
					"other": map[string]interface{}{
						"nested": "something",
					},
				},
				overrides: map[string]string{
					"other.nested": "somethingElse",
				},
			},
			params: map[string]interface{}{
				"test": "value",
				"other": map[string]interface{}{
					"nested": "somethingElse",
				},
			},
		},
		"SuccessfulOverrides": {
			reason: "If no base is provided just overrides should be returned.",
			parser: &Parser{
				values: map[string]interface{}{},
				overrides: map[string]string{
					"other.nested": "somethingElse",
				},
			},
			params: map[string]interface{}{
				"other": map[string]interface{}{
					"nested": "somethingElse",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := tc.parser.Parse()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.params, p); diff != "" {
				t.Errorf("\n%s\nParse(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
