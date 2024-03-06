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

package views

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/prometheus/common/expfmt"
)

const dataLength = 1000

type ExampleGraph struct {
	*tvxwidgets.Sparkline

	client  http.RoundTripper
	kubeURL string
	data    []float64
}

func NewExampleGraph(client http.RoundTripper, kubeURL string) *ExampleGraph {
	d := &ExampleGraph{
		Sparkline: tvxwidgets.NewSparkline(),
		client:    client,
		kubeURL:   kubeURL,
		data:      []float64{0},
	}
	d.Sparkline.SetBorder(false)
	d.Sparkline.SetLineColor(tcell.ColorMediumPurple)

	return d
}

func (g *ExampleGraph) Tick(ctx context.Context) error {
	req, err := http.NewRequest("GET", strings.TrimSuffix(g.kubeURL, "/")+"/metrics", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := g.client.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get metrics: %s", resp.Status)
	}
	defer resp.Body.Close() // nolint:errcheck

	// Parse the metrics
	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse metrics: %w", err)
	}

	// Process the relevant metrics to calculate the average request duration
	var sumDuration float64
	var count float64
	if sum, ok := metrics["apiserver_request_duration_seconds"]; ok {
	nextMetric:
		for _, m := range sum.Metric {
			for _, l := range m.GetLabel() {
				if l.GetName() == "verb" && l.GetValue() == "WATCH" {
					continue nextMetric
				}
				if l.GetName() == "subresource" && l.GetValue() != "" {
					continue nextMetric
				}
			}
			sumDuration += m.GetHistogram().GetSampleSum()
			count += float64(m.GetHistogram().GetSampleCount())
		}
	}
	var v float64
	if count > 0 {
		v = sumDuration / count
	}

	if len(g.data) < dataLength {
		g.data = append(g.data, v)
	} else {
		g.data = append(g.data[:dataLength-1], v)
	}
	g.Sparkline.SetData(g.data)
	g.Sparkline.SetDataTitle(fmt.Sprintf("Average Request Duration of kube-apiserver: %.2fms", v*1000.0))

	return nil
}
