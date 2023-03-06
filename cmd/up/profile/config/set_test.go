// Copyright 2022 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func TestSetValidateInput(t *testing.T) {
	tf, _ := os.CreateTemp("", "")

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
			reason: "Supplying no k and v, and just file is valid.",
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
