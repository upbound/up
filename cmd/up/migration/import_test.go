// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migration

import (
	"testing"
)

func TestIsMCP(t *testing.T) {
	type args struct {
		host string
	}
	tests := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"New": {
			reason: "Should match with the new format",
			args: args{
				host: "https://00.000.000.0.nip.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s",
			},
			want: true,
		},
		"OldLower": {
			reason: "Should match with the old format with lowercase controplanes",
			args: args{
				host: "https://spaces-foo.upboundrocks.cloud/v1/controlplanes/acmeco/default/ctp/k8s",
			},
			want: true,
		},
		"OldCamelCase": {
			reason: "Should match with the old format with camelcase controlPlanes",
			args: args{
				host: "https://spaces-foo.upboundrocks.cloud/v1/controlPlanes/acmeco/default/ctp/k8s",
			},
			want: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := isMCP(tt.args.host); got != tt.want {
				t.Errorf("isMCP() = %v, want %v", got, tt.want)
			}
		})
	}
}
