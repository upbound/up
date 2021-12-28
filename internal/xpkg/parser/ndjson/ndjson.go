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

package ndjson

import (
	"bufio"
	"io"
)

// Reader defines the read API for the underlying reader for the
// NDJSONReader.
type Reader interface {
	Read() ([]byte, error)
}

// JSONReader represents a newline delimited JSON reader.
type JSONReader struct {
	reader Reader
}

// NewReader returns a new reader, using the underlying io.Reader
// as input.
func NewReader(r *bufio.Reader) *JSONReader {
	return &JSONReader{
		reader: &LineReader{reader: r},
	}
}

// Read returns a full JSON document.
func (r *JSONReader) Read() ([]byte, error) {
	line, err := r.reader.Read()
	if err != nil && err != io.EOF {
		return nil, err
	}

	if len(line) != 0 {
		return line, nil
	}

	// EOF seen and there's nothing left in the reader, return EOF.
	return nil, err
}

// LineReader represents a reader that reads from the underlying reader
// line by line, separated by '\n'.
type LineReader struct {
	reader *bufio.Reader
}

// Read returns a single line (with '\n' ended) from the underlying reader.
// An error is returned iff there is an error with the underlying reader.
func (r *LineReader) Read() ([]byte, error) {
	return r.reader.ReadBytes('\n')
}
