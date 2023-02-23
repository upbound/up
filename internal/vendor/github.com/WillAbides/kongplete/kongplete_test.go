package kongplete

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	envLine  = "COMP_LINE"
	envPoint = "COMP_POINT"
)

func TestComplete(t *testing.T) {
	type embed struct {
		Lion string
	}

	predictors := map[string]complete.Predictor{
		"things":      complete.PredictSet("thing1", "thing2"),
		"otherthings": complete.PredictSet("otherthing1", "otherthing2"),
	}

	var cli struct {
		Foo struct {
			Embedded embed  `kong:"embed"`
			Bar      string `kong:"predictor=things"`
			Baz      bool
			Qux      bool     `kong:"hidden"`
			Rabbit   struct{} `kong:"cmd"`
			Duck     struct{} `kong:"cmd"`
		} `kong:"cmd"`
		Bar struct {
			Tiger   string `kong:"arg,predictor=things"`
			Bear    string `kong:"arg,predictor=otherthings"`
			OMG     string `kong:"required,enum='oh,my,gizzles'"`
			Number  int    `kong:"required,short=n,enum='1,2,3'"`
			BooFlag bool   `kong:"name=boofl,short=b"`
		} `kong:"cmd"`
		Baz struct{} `kong:"cmd,hidden"`
	}

	for _, td := range []completeTest{
		{
			parser: kong.Must(&cli),
			want:   []string{"foo", "bar"},
			line:   "myApp ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"foo"},
			line:   "myApp foo",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"rabbit", "duck"},
			line:   "myApp foo ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"rabbit"},
			line:   "myApp foo r",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"--bar", "--baz", "--lion", "--help", "-h"},
			line:   "myApp foo -",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{},
			line:   "myApp foo --lion ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"rabbit", "duck"},
			line:   "myApp foo --baz ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"--bar", "--baz", "--lion", "--help", "-h"},
			line:   "myApp foo --baz -",
		},
		{
			parser: kong.Must(&cli),

			want: []string{"thing1", "thing2"},
			line: "myApp foo --bar ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"thing1", "thing2"},
			line:   "myApp bar ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"thing1", "thing2"},
			line:   "myApp bar thing",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"otherthing1", "otherthing2"},
			line:   "myApp bar thing1 ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"oh", "my", "gizzles"},
			line:   "myApp bar --omg ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"-n", "--number", "--omg", "--help", "-h", "--boofl", "-b"},
			line:   "myApp bar -",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"thing1", "thing2"},
			line:   "myApp bar -b ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"-n", "--number", "--omg", "--help", "-h", "--boofl", "-b"},
			line:   "myApp bar -b thing1 -",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"oh", "my", "gizzles"},
			line:   "myApp bar -b thing1 --omg ",
		},
		{
			parser: kong.Must(&cli),
			want:   []string{"otherthing1", "otherthing2"},
			line:   "myApp bar -b thing1 --omg gizzles ",
		},
	} {
		name := td.name
		if name == "" {
			name = td.line
		}
		t.Run(name, func(t *testing.T) {
			options := []Option{WithPredictors(predictors)}
			got := runComplete(t, td.parser, td.line, options)
			assert.ElementsMatch(t, td.want, got)
		})
	}
}

func Test_tagPredictor(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got, err := tagPredictor(nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("no predictor tag", func(t *testing.T) {
		got, err := tagPredictor(testTag{}, nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("missing predictor", func(t *testing.T) {
		got, err := tagPredictor(testTag{predictorTag: "foo"}, nil)
		assert.Error(t, err)
		assert.Equal(t, `no predictor with name "foo"`, err.Error())
		assert.Nil(t, got)
	})

	t.Run("existing predictor", func(t *testing.T) {
		got, err := tagPredictor(testTag{predictorTag: "foo"}, map[string]complete.Predictor{"foo": complete.PredictAnything})
		assert.NoError(t, err)
		assert.NotNil(t, got)
	})
}

type testTag map[string]string

func (t testTag) Has(k string) bool {
	_, ok := t[k]
	return ok
}

func (t testTag) Get(k string) string {
	return t[k]
}

type completeTest struct {
	name   string
	parser *kong.Kong
	want   []string
	line   string
}

func setLineAndPoint(t *testing.T, line string) func() {
	t.Helper()
	origLine, hasOrigLine := os.LookupEnv(envLine)
	origPoint, hasOrigPoint := os.LookupEnv(envPoint)
	require.NoError(t, os.Setenv(envLine, line))
	require.NoError(t, os.Setenv(envPoint, strconv.Itoa(len(line))))
	return func() {
		t.Helper()
		require.NoError(t, os.Unsetenv(envLine))
		require.NoError(t, os.Unsetenv(envPoint))
		if hasOrigLine {
			require.NoError(t, os.Setenv(envLine, origLine))
		}
		if hasOrigPoint {
			require.NoError(t, os.Setenv(envPoint, origPoint))
		}
	}
}

func runComplete(t *testing.T, parser *kong.Kong, line string, options []Option) []string {
	t.Helper()
	options = append(options,
		WithErrorHandler(func(err error) {
			t.Helper()
			assert.NoError(t, err)
		}),
		WithExitFunc(func(code int) {
			t.Helper()
			assert.Equal(t, 0, code)
		}),
	)
	cleanup := setLineAndPoint(t, line)
	defer cleanup()
	var buf bytes.Buffer
	if parser != nil {
		parser.Stdout = &buf
	}
	Complete(parser, options...)
	return parseOutput(buf.String())
}

func parseOutput(output string) []string {
	lines := strings.Split(output, "\n")
	options := []string{}
	for _, l := range lines {
		if l != "" {
			options = append(options, l)
		}
	}
	return options
}
