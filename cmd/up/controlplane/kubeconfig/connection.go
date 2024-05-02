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

package kubeconfig

// ConnectionSecretCmd is the base for command getting connection secret for a control plane.
type ConnectionSecretCmd struct {
	Name  string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
	Token string `help:"API token used to authenticate. Required for Upbound Cloud; ignored otherwise."`
	Group string `short:"g" help:"The control plane group that the control plane is contained in. By default, this is the group specified in the current profile."`
}
