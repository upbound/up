package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInputYes(t *testing.T) {

	type args struct {
		input string
	}

	type want struct {
		output bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InputYes": {
			reason: "We should see true for 'Yes'",
			args: args{
				input: "Yes",
			},
			want: want{
				output: true,
			},
		},
		"InputNo": {
			reason: "We should see false for 'No'",
			args: args{
				input: "No",
			},
			want: want{
				output: false,
			},
		},
		"InputUmm": {
			reason: "We should see false for 'umm'",
			args: args{
				input: "umm",
			},
			want: want{
				output: false,
			},
		},
		"InputEmpty": {
			reason: "We should see false for ''",
			args: args{
				input: "",
			},
			want: want{
				output: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			y := inputYes(tc.args.input)

			if diff := cmp.Diff(tc.want.output, y); diff != "" {
				t.Errorf("\n%s\nInputYes(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
