// Copyright 2021 Upbound Inc
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

package kube

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const errClosedResults = "stopped watching before condition met"

// DynamicWatch starts a watch on the given resource type. The done callback is
// called on every received event until either timeout or context cancellation.
func DynamicWatch(ctx context.Context, r dynamic.NamespaceableResourceInterface, timeout *int64, done func(u *unstructured.Unstructured) (bool, error)) (chan error, error) {
	w, err := r.Watch(ctx, v1.ListOptions{
		TimeoutSeconds: timeout,
	})
	if err != nil {
		return nil, err
	}
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		for {
			select {
			case e, ok := <-w.ResultChan():
				// If we are no longer watching return with error.
				if !ok {
					errChan <- errors.New(errClosedResults)
					return
				}

				u, ok := e.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				// If we error on event callback return early.
				d, err := done(u)
				if err != nil {
					errChan <- err
					return
				}
				// If event callback indicated done, return early with nil
				// error.
				if d {
					errChan <- nil
					return
				}
			// If context is canceled, stop watching and return error.
			case <-ctx.Done():
				w.Stop()
				errChan <- ctx.Err()
				return
			}
		}
	}()
	return errChan, nil
}
