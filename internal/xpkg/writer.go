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

package xpkg

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	errAlreadyExists    = "directory contains pre-existing meta file"
	errRootDoesNotExist = "target directory does not exist"
)

// Writer defines a writer that is used for creating package meta files.
type Writer struct {
	createRoot bool
	fileBody   []byte
	fs         afero.Fs
	root       string
}

// NewFileWriter returns a new Writer.
func NewFileWriter(opts ...Option) *Writer {
	w := &Writer{}

	for _, o := range opts {
		o(w)
	}

	return w
}

// Option modifies the Writer.
type Option func(*Writer)

// WithCreateRoot specifies whether or not to create the configured root
// directory if it does not yet exist.
func WithCreateRoot(create bool) Option {
	return func(w *Writer) {
		w.createRoot = create
	}
}

// WithFs specifies the afero.Fs that is being used.
func WithFs(fs afero.Fs) Option {
	return func(w *Writer) {
		w.fs = fs
	}
}

// WithRoot specifies the root for the new package.
func WithRoot(root string) Option {
	return func(w *Writer) {
		w.root = root
	}
}

// WithFileBody specifies the file body that is used to populate
// the new meta file.
func WithFileBody(body []byte) Option {
	return func(w *Writer) {
		w.fileBody = body
	}
}

// NewMetaFile creates a new meta file per the given options.
func (w *Writer) NewMetaFile() error {
	targetFile := filepath.Join(w.root, MetaFile)

	// return err if file already exists
	exists, err := afero.Exists(w.fs, targetFile)
	if err != nil {
		return err
	}
	if exists {
		return errors.New(errAlreadyExists)
	}

	// return err if directory does not exist and we're not instructed to create it
	exists, err = afero.DirExists(w.fs, w.root)
	if err != nil {
		return err
	}

	if !exists && !w.createRoot {
		return errors.New(errRootDoesNotExist)
	}

	if !exists && w.createRoot {
		if err := w.fs.MkdirAll(w.root, os.ModePerm); err != nil {
			return err
		}
	}

	return afero.WriteFile(w.fs, targetFile, w.fileBody, StreamFileMode)
}
