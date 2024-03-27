// Copyright 2021 Google LLC
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

package ctx

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func kubectxPrevCtxFile() (string, error) {
	home, err := os.UserHomeDir()
	if home == "" || err != nil {
		return "", errors.New("HOME or USERPROFILE environment variable not set")
	}
	return filepath.Join(home, ".kube", "kubectx"), nil
}

// readLastContext returns the saved previous context
// if the state file exists, otherwise returns "".
func readLastContext() (string, error) {
	path, err := kubectxPrevCtxFile()
	if err != nil {
		return "", err
	}
	bs, err := os.ReadFile(path) // nolint:gosec // it's ok
	if os.IsNotExist(err) {
		return "", nil
	} // nolint:gosec // it's ok
	return string(bs), err
}

// writeLastContext saves the specified value to the state file.
// It creates missing parent directories.
func writeLastContext(value string) error {
	path, err := kubectxPrevCtxFile()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil { // nolint:gosec // it's ok
		return errors.Wrap(err, "failed to create parent directories")
	}
	return os.WriteFile(path, []byte(value), 0644) // nolint:gosec // it's ok
}
