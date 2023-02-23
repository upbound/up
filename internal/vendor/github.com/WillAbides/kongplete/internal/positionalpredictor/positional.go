package positionalpredictor

import (
	"strings"

	"github.com/posener/complete"
)

// PositionalPredictor is a predictor for positional arguments
type PositionalPredictor struct {
	Predictors []complete.Predictor
	ArgFlags   []string
	BoolFlags  []string
}

// Predict implements complete.Predict
func (p *PositionalPredictor) Predict(a complete.Args) []string {
	predictor := p.predictor(a)
	if predictor == nil {
		return []string{}
	}
	return predictor.Predict(a)
}

func (p *PositionalPredictor) predictor(a complete.Args) complete.Predictor {
	position := p.predictorIndex(a)
	complete.Log("predicting positional argument(%d)", position)
	if position < 0 || position > len(p.Predictors)-1 {
		return nil
	}
	return p.Predictors[position]
}

// predictorIndex returns the index in predictors to use. Returns -1 if no predictor should be used.
func (p *PositionalPredictor) predictorIndex(a complete.Args) int {
	idx := 0
	for i := 0; i < len(a.Completed); i++ {
		if !p.nonPredictorPos(a, i) {
			idx++
		}
	}
	return idx
}

// nonPredictorPos returns true if the value at this position is either a flag or a flag's argument
func (p *PositionalPredictor) nonPredictorPos(a complete.Args, pos int) bool {
	if pos < 0 || pos > len(a.All)-1 {
		return false
	}
	val := a.All[pos]
	if p.valIsFlag(val) {
		return true
	}
	if pos == 0 {
		return false
	}
	prev := a.All[pos-1]
	return p.nextValueIsFlagArg(prev)
}

// valIsFlag returns true if the value matches a flag from the configuration
func (p *PositionalPredictor) valIsFlag(val string) bool {
	val = strings.Split(val, "=")[0]
	for _, flag := range p.BoolFlags {
		if flag == val {
			return true
		}
	}
	for _, flag := range p.ArgFlags {
		if flag == val {
			return true
		}
	}
	return false
}

// nextValueIsFlagArg returns true if the value matches an ArgFlag and doesn't contain an equal sign.
func (p *PositionalPredictor) nextValueIsFlagArg(val string) bool {
	if strings.Contains(val, "=") {
		return false
	}
	for _, flag := range p.ArgFlags {
		if flag == val {
			return true
		}
	}
	return false
}
