package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func TestValidateInput(t *testing.T) {
	tf, _ := ioutil.TempFile("", "")

	type args struct {
		key   string
		value string
		file  *os.File
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"KeyNoValue": {
			reason: "Supplying a key, but no value is invalid.",
			args: args{
				key: "k",
			},
			want: want{
				err: errors.New(errOnlyKVFileXOR),
			},
		},
		"KeyValueAndFile": {
			reason: "Supplying a key, value, and file is invalid.",
			args: args{
				key:   "k",
				value: "v",
				file:  tf,
			},
			want: want{
				err: errors.New(errOnlyKVFileXOR),
			},
		},
		"KeyValueNoFile": {
			reason: "Supplying k and v, and no file is valid.",
			args: args{
				key:   "k",
				value: "v",
			},
			want: want{},
		},
		"FileNoKeyValue": {
			reason: "Supplying k and v, and no file is valid.",
			args: args{
				file: tf,
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := &setCmd{
				Key:   tc.args.key,
				Value: tc.args.value,
				File:  tc.args.file,
			}

			err := c.validateInput()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateInput(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
