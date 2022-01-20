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

package xpls

import "os"

// StdRWC is a readwritecloser on stdio, which can be used as a JSON-RPC
// transport.
type StdRWC struct{}

// Read reads from stdin.
func (StdRWC) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

// Write writes to stdout.
func (StdRWC) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

// Close first closes stdin, then, if successful, closes stdout.
func (StdRWC) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
