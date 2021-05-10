package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/strvals"

	"github.com/upbound/up/internal/uxp"
)

// Parser is a helm-style parameter parser.
type Parser struct {
	values    map[string]interface{}
	overrides map[string]string
}

// NewParser returns a parameter parser backed by helm.
func NewParser(base map[string]interface{}, overrides map[string]string) uxp.ParameterParser {
	return &Parser{
		values:    base,
		overrides: overrides,
	}
}

// Parse parses install and upgrade parameters
func (p *Parser) Parse() (map[string]interface{}, error) {
	for k, v := range p.overrides {
		if err := strvals.ParseInto(fmt.Sprintf("%s=%s", k, v), p.values); err != nil {
			return nil, err
		}
	}
	return p.values, nil
}
