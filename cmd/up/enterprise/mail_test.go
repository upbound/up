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

package enterprise

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	ktesting "k8s.io/client-go/testing"
)

func withSecretReactor(verb string, fRet runtime.Object, fErr error) *fake.Clientset {
	c := fake.NewSimpleClientset()
	c.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor(verb, "secrets", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, fRet, fErr
	})
	return c
}

func TestHandler(t *testing.T) {
	emptySec := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-secret-name",
		},
	}
	validSec := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-secret-name",
		},
		Data: map[string][]byte{
			mailMessageKey: []byte("MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\nFrom: me\r\nTo: you\r\nSubject: hello\r\n\r\nhello world\r\n"),
		},
	}
	errBoom := errors.New("boom")
	b, _ := f.ReadFile(mailTmplPath)
	tmpl, _ := template.New("mail").Parse(string(b))
	type want struct {
		status int
	}

	cases := map[string]struct {
		reason   string
		m        *mailCmd
		endpoint string
		want     want
	}{
		"ErrorListSecrets": {
			reason: "Should return an error if unable to list secrets.",
			m: &mailCmd{
				log:     logging.NewNopLogger(),
				kClient: withSecretReactor("list", &v1.SecretList{}, errBoom),
			},
			endpoint: "/",
			want: want{
				status: http.StatusInternalServerError,
			},
		},
		"SuccessInvalidMessage": {
			reason: "Should not return an error if invalid message encountered in list.",
			m: &mailCmd{
				log: logging.NewNopLogger(),
				kClient: withSecretReactor("list", &v1.SecretList{
					Items: []v1.Secret{
						emptySec,
					},
				}, nil),
				tmpl: tmpl,
			},
			endpoint: "/",
			want: want{
				status: http.StatusOK,
			},
		},
		"SuccessValidMessages": {
			reason: "Should not return an error if messages are valid.",
			m: &mailCmd{
				log: logging.NewNopLogger(),
				kClient: withSecretReactor("list", &v1.SecretList{
					Items: []v1.Secret{
						validSec,
					},
				}, nil),
				tmpl: tmpl,
			},
			endpoint: "/",
			want: want{
				status: http.StatusOK,
			},
		},
		"ErrorGetSecret": {
			reason: "Should return an error if unable to get secret.",
			m: &mailCmd{
				log:     logging.NewNopLogger(),
				kClient: withSecretReactor("get", &v1.Secret{}, errBoom),
			},
			endpoint: "/cool-secret-name",
			want: want{
				status: http.StatusInternalServerError,
			},
		},
		"SuccessValidMessage": {
			reason: "Should not return an error if message is valid.",
			m: &mailCmd{
				log:     logging.NewNopLogger(),
				kClient: withSecretReactor("get", &validSec, nil),
			},
			endpoint: "/cool-secret-name",
			want: want{
				status: http.StatusOK,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req, err := http.NewRequestWithContext(context.Background(), "GET", tc.endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}
			http.HandlerFunc(tc.m.handler).ServeHTTP(rr, req)
			if diff := cmp.Diff(tc.want.status, rr.Code, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nhandle(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
