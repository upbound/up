package positionalpredictor

import (
	"strings"
	"testing"
	"unicode"

	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestPositionalPredictor_position(t *testing.T) {
	posPredictor := &PositionalPredictor{
		BoolFlags: []string{"--mybool", "-b"},
		ArgFlags:  []string{"--myarg", "-a"},
	}

	for args, want := range map[string]int{
		``:                 0,
		`foo`:              0,
		`foo `:             1,
		`-b foo `:          1,
		`-a foo `:          0,
		`-a=omg foo `:      1,
		`--myarg omg foo `: 1,
		`--myarg=omg foo `: 1,
		`foo bar`:          1,
		`foo bar `:         2,
	} {
		t.Run(args, func(t *testing.T) {
			got := posPredictor.predictorIndex(newArgs("foo " + args))
			assert.Equal(t, want, got)
		})
	}
}

func TestPositionalPredictor_predictor(t *testing.T) {
	predictor1 := complete.PredictSet("1")
	predictor2 := complete.PredictSet("2")
	posPredictor := &PositionalPredictor{
		Predictors: []complete.Predictor{predictor1, predictor2},
	}

	for args, want := range map[string]complete.Predictor{
		``:         predictor1,
		`foo`:      predictor1,
		`foo `:     predictor2,
		`foo bar`:  predictor2,
		`foo bar `: nil,
	} {
		t.Run(args, func(t *testing.T) {
			got := posPredictor.predictor(newArgs("app " + args))
			assert.Equal(t, want, got)
		})
	}
}

// The code below is taken from https://github.com/posener/complete/blob/f6dd29e97e24f8cb51a8d4050781ce2b238776a4/args.go
// to assist in tests.

func newArgs(line string) complete.Args {
	var (
		all       []string
		completed []string
	)
	parts := splitFields(line)
	if len(parts) > 0 {
		all = parts[1:]
		completed = removeLast(parts[1:])
	}
	return complete.Args{
		All:           all,
		Completed:     completed,
		Last:          last(parts),
		LastCompleted: last(completed),
	}
}

// splitFields returns a list of fields from the given command line.
// If the last character is space, it appends an empty field in the end
// indicating that the field before it was completed.
// If the last field is of the form "a=b", it splits it to two fields: "a", "b",
// So it can be completed.
func splitFields(line string) []string {
	parts := strings.Fields(line)

	// Add empty field if the last field was completed.
	if len(line) > 0 && unicode.IsSpace(rune(line[len(line)-1])) {
		parts = append(parts, "")
	}

	// Treat the last field if it is of the form "a=b"
	parts = splitLastEqual(parts)
	return parts
}

func splitLastEqual(line []string) []string {
	if len(line) == 0 {
		return line
	}
	parts := strings.Split(line[len(line)-1], "=")
	return append(line[:len(line)-1], parts...)
}

func removeLast(a []string) []string {
	if len(a) > 0 {
		return a[:len(a)-1]
	}
	return a
}

func last(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}
