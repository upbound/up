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

package oci

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
)

// MockPathNavigator is a mock implementation of PathNavigator for testing
type MockPathNavigator struct {
	SupportedVersions []string `json:"supportedVersions"`
}

func (m *MockPathNavigator) GetSupportedVersions() ([]string, error) {
	return m.SupportedVersions, nil
}

// Mock functions
func mockLoaderLoad(name string) (*chart.Chart, error) {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name: name,
		},
		Values: map[string]interface{}{
			"supportedVersions": []string{"v1", "v2", "v3"},
		},
	}, nil
}

func mockPullRun(src, version string) (string, error) {
	dir, err := os.MkdirTemp("", "mock")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	return dir, nil
}

// TestGetValuesFromChart tests the GetValuesFromChart function
func TestGetValuesFromChart(t *testing.T) {
	// Setup mock functions and data
	chartName := "test-chart"
	version := "1.0.0"
	supportedVersions := []string{"v1", "v2", "v3"}

	mockNavigator := &MockPathNavigator{}
	mockNavigator.SupportedVersions = supportedVersions

	// Run the function
	versions, err := getValuesFromChartWithLoaderAndPull(chartName, version, mockNavigator, mockLoaderLoad, mockPullRun)
	require.NoError(t, err)
	assert.Equal(t, supportedVersions, versions)
}

// TestGetValuesFromChart_PullError tests the GetValuesFromChart function when Pull fails
func TestGetValuesFromChart_PullError(t *testing.T) {
	// Setup mock functions and data
	chartName := "test-chart"
	version := "1.0.0"

	mockNavigator := &MockPathNavigator{}

	// Mock Pull function to simulate an error
	mockPullRunError := func(src, version string) (string, error) {
		return "", fmt.Errorf("failed to pull chart")
	}

	// Run the function
	_, err := getValuesFromChartWithLoaderAndPull(chartName, version, mockNavigator, mockLoaderLoad, mockPullRunError)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pull chart")
}

// TestGetValuesFromChart_LoadError tests the GetValuesFromChart function when Load fails
func TestGetValuesFromChart_LoadError(t *testing.T) {
	// Setup mock functions and data
	chartName := "test-chart"
	version := "1.0.0"

	mockNavigator := &MockPathNavigator{}

	// Mock the loader function to simulate an error
	loaderFuncError := func(name string) (*chart.Chart, error) {
		return nil, fmt.Errorf("failed to load chart")
	}

	// Run the function
	_, err := getValuesFromChartWithLoaderAndPull(chartName, version, mockNavigator, loaderFuncError, mockPullRun)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load chart")
}

// TestGetValuesFromChart_JSONError tests the GetValuesFromChart function when JSON unmarshalling fails
func TestGetValuesFromChart_JSONError(t *testing.T) {
	// Setup mock functions and data
	chartName := "test-chart"
	version := "1.0.0"

	mockNavigator := &MockPathNavigator{}

	// Mock chart values with invalid JSON
	loaderFuncError := func(name string) (*chart.Chart, error) {
		return &chart.Chart{
			Metadata: &chart.Metadata{
				Name: name,
			},
			Values: map[string]interface{}{
				"supportedVersions": func() {}, // Invalid type for JSON marshaling
			},
		}, nil
	}

	// Run the function
	_, err := getValuesFromChartWithLoaderAndPull(chartName, version, mockNavigator, loaderFuncError, mockPullRun)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal chart values")
}
