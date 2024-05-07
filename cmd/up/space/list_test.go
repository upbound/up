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

package space

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up-sdk-go"
	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	uerrors "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/fake"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

func TestListCommand(t *testing.T) {
	acc := "some-account"
	errBoom := errors.New("boom")
	accResp := `{"account": {"name": "some-account", "type": "organization"}, "organization": {"name": "some-account"}}`

	type args struct {
		cmd   *listCmd
		upCtx *upbound.Context
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotLoggedIntoCloud": {
			reason: "If the user is not logged into cloud, we expect no error and instead a nice human message.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: fake.NewMockDoFn(&uerrors.Error{
								Status: http.StatusUnauthorized,
							}),
						},
					}),
				},
				upCtx: &upbound.Context{
					Account: "some-account",
				},
			},
			want: want{},
		},
		"ErrFailedToQueryForAccount": {
			reason: "If the user could not query cloud due to service availability we should return an error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: fake.NewMockDoFn(&uerrors.Error{
								Status: http.StatusServiceUnavailable,
								Title:  http.StatusText(http.StatusServiceUnavailable),
							}),
						},
					}),
				},
				upCtx: &upbound.Context{
					Account: acc,
				},
			},
			want: want{
				err: errors.Wrap(errors.New(`failed to get Account "some-account": Service Unavailable`), errListSpaces),
			},
		},
		"ErrFailedToListSpaces": {
			reason: "If we received an error from Cloud, we should return an error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(req *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: test.NewMockListFn(errBoom),
					},
				},
				upCtx: &upbound.Context{
					Account: acc,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListSpaces),
			},
		},
		"NoSpacesFound": {
			reason: "If we were able to query Cloud, but no spaces were found, we should print a human consumable error.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(req *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: test.NewMockListFn(nil),
					},
				},
				upCtx: &upbound.Context{
					Account: acc,
				},
			},
			want: want{},
		},
		"SpacesFound": {
			reason: "If we were able to query Cloud, we should attempt to print what was found.",
			args: args{
				cmd: &listCmd{
					ac: accounts.NewClient(&up.Config{
						Client: &fake.MockClient{
							MockNewRequest: fake.NewMockNewRequestFn(nil, nil),
							MockDo: func(req *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte(accResp), &obj)
							},
						},
					}),
					kc: &test.MockClient{
						MockList: func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
							list := obj.(*upboundv1alpha1.SpaceList)
							list.Items = []upboundv1alpha1.Space{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "some-space",
									},
								},
							}
							return nil
						},
					},
				},
				upCtx: &upbound.Context{
					Account: acc,
				},
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			err := tc.args.cmd.Run(
				context.Background(),
				upterm.NewNopObjectPrinter(),
				upterm.NewNopTextPrinter(),
				tc.args.upCtx,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateInput(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
