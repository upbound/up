// Copyright 2023 Upbound Inc
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

package json

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/upbound/up/internal/usage/model"
)

// MCPGVKEventDecoder decodes MCP GVK events from a reader containing a JSON
// array of event objects. Must be initialized with NewMCPGVKEventDecoder().
type MCPGVKEventDecoder struct {
	jd *json.Decoder
}

// NewMCPGVKEventDecoder returns an initialized *Decoder.
func NewMCPGVKEventDecoder(r io.Reader) (*MCPGVKEventDecoder, error) {
	jd := json.NewDecoder(r)

	// Consume open bracket from JSON array.
	t, err := jd.Token()
	if err != nil {
		return nil, fmt.Errorf("reader does not contain valid JSON: %s", err.Error())
	}
	if t != json.Delim('[') {
		return nil, fmt.Errorf("reader does not contain JSON array. expected [, got %s", t)
	}

	return &MCPGVKEventDecoder{jd: jd}, nil
}

// More returns true if there is more input to be decoded.
func (d *MCPGVKEventDecoder) More() bool {
	return d.jd.More()
}

// Decode returns the next MCP GVK event from input.
func (d *MCPGVKEventDecoder) Decode() (model.MCPGVKEvent, error) {
	var e model.MCPGVKEvent
	err := d.jd.Decode(&e)
	if err != nil {
		return model.MCPGVKEvent{}, fmt.Errorf("error decoding next event: %s", err.Error())
	}
	return e, nil
}
