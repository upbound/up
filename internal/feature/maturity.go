package feature

import (
	"github.com/alecthomas/kong"
)

// maturityTag is the struct field tag used to specify maturity of a command.
const maturityTag = "maturity"

// Maturity is the maturity of a feature.
type Maturity string

// Currently supported maturity levels.
const (
	Alpha  Maturity = "alpha"
	Stable Maturity = "stable"
)

// HideMaturity hides commands that are not at the specified level of maturity.
func HideMaturity(p *kong.Path, maturity Maturity) error {
	children := p.Node().Children
	for _, c := range children {
		mt := Maturity(c.Tag.Get(maturityTag))
		if mt == "" {
			mt = Stable
		}
		if mt != maturity {
			c.Hidden = true
		}
	}
	return nil
}

// GetMaturity gets the maturity of the node.
func GetMaturity(n *kong.Node) Maturity {
	if m := Maturity(n.Tag.Get(maturityTag)); m != "" {
		return m
	}
	return Stable
}
