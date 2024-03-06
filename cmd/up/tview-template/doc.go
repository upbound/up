// Copyright 2024 Upbound Inc
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

// Package template is meant to be a template for creation text ui commands. For
// a new command, copy this folder to cmd/up/<your-copy> and adapt it to your
// needs.
//
// This template intentionally follows a certain structure to make it easier to
// build maintainable text uis. Especially the use of a model and views rendering
// the model is highly encouraged.
//
// See https://github.com/rivo/tview for more information on tview.
//
// There is also the internal/tview package which means to serve as a collection
// of shared code between different tview commands. Please consider moving
// reusable components there.

package template
