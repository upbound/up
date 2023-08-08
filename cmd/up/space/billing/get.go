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

package billing

import (
	"archive/tar"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage"
	"github.com/upbound/up/internal/usage/report"
	reportaws "github.com/upbound/up/internal/usage/report/aws"
	reporttar "github.com/upbound/up/internal/usage/report/file/tar"
	reportgcs "github.com/upbound/up/internal/usage/report/gcs"
)

const (
	providerAWS   = "aws"
	providerGCP   = "gcp"
	providerAzure = "azure"

	errFmtProviderNotSupported = "%q is not supported"
)

type dateRange usage.TimeRange

func (d *dateRange) Decode(ctx *kong.DecodeContext) error {
	var value string
	if err := ctx.Scan.PopValueInto("date range", &value); err != nil {
		return err
	}

	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		fmt.Printf("%s\n", parts)
		return fmt.Errorf("invalid format")
	}

	start, err := time.Parse(time.DateOnly, parts[0])
	if err != nil {
		return err
	}
	d.Start = start

	end, err := time.Parse(time.DateOnly, parts[1])
	if err != nil {
		return err
	}
	d.End = end

	return nil
}

type provider string

func (p provider) Validate() error {
	// TODO(branden): Add support Azure.
	switch p {
	case providerGCP:
		return nil
	case providerAWS:
		return nil
	default:
		return fmt.Errorf(errFmtProviderNotSupported, p)
	}
}

type getCmd struct {
	Out string `optional:"" short:"o" env:"UP_BILLING_OUT" default:"upbound_billing_report.tgz" help:"Name of the output file."`

	// TODO(branden): Make storage params optional and fetch missing values from spaces cluster.
	Provider provider `required:"" enum:"aws,gcp,azure," env:"UP_BILLING_PROVIDER" group:"Storage" help:"Storage provider. Must be one of: aws, gcp, azure."`
	Bucket   string   `required:"" env:"UP_BILLING_BUCKET" group:"Storage" help:"Storage bucket."`
	Endpoint string   `env:"UP_BILLING_ENDPOINT" group:"Storage" help:"Custom storage endpoint."`
	Account  string   `required:"" env:"UP_BILLING_ACCOUNT" group:"Storage" help:"Name of the Upbound account whose billing report is being collected."`

	BillingMonth    time.Time  `format:"2006-01" required:"" xor:"billingperiod" env:"UP_BILLING_MONTH" group:"Billing period" help:"Get a report for a billing period of one calendar month. Format: 2006-01."`
	BillingCustom   *dateRange `required:"" xor:"billingperiod" env:"UP_BILLING_CUSTOM" group:"Billing period" help:"Get a report for a custom billing period. Date range is inclusive. Format: 2006-01-02/2006-01-02."`
	ForceIncomplete bool       `env:"UP_BILLING_FORCE_INCOMPLETE" group:"Billing period" help:"Get a report for an incomplete billing period."`

	outAbs        string
	billingPeriod usage.TimeRange
}

//go:embed get_help.txt
var getCmdHelp string

func (c *getCmd) Help() string {
	return getCmdHelp
}

func (c *getCmd) Validate() error {
	// Get billing period.
	var err error
	c.billingPeriod, err = c.getBillingPeriod()
	if err != nil {
		return errors.Wrap(err, "error getting billing period")
	}

	// Validate billing period.
	now := time.Now()
	if !c.ForceIncomplete && c.billingPeriod.Start.Before(now) && c.billingPeriod.End.After(now) {
		return fmt.Errorf("billing period is incomplete, use --force-incomplete to continue")
	}

	// Validate output filename.
	c.outAbs, err = filepath.Abs(c.Out)
	if err != nil {
		return err
	}
	_, err = os.Stat(c.outAbs)
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("file \"%s\" already exists", c.Out)
	}
	return nil
}

func (c *getCmd) Run() error {
	fmt.Printf(
		"Getting billing report for Upbound account %s from %s to %s.\n",
		c.Account,
		formatTimestamp(c.billingPeriod.Start),
		formatTimestamp(c.billingPeriod.End),
	)
	fmt.Printf("\n")
	fmt.Printf("Reading usage data from storage...\n")
	fmt.Printf("Provider: %s\n", c.Provider)
	fmt.Printf("Bucket: %s\n", c.Bucket)
	if c.Endpoint != "" {
		fmt.Printf("Endpoint: %s\n", c.Endpoint)
	}

	if err := c.collectReport(); err != nil {
		c.cleanupOnError()
		return err
	}

	fmt.Printf("\n")
	fmt.Printf("Billing report saved to %s\n", c.outAbs)
	return nil
}

func (c *getCmd) cleanupOnError() {
	if err := os.Remove(c.outAbs); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up: %s", err)
	}
}

func (c *getCmd) collectReport() error {
	f, err := os.Create(c.outAbs)
	if err != nil {
		return errors.Wrap(err, "error creating report")
	}
	defer f.Close() // nolint:errcheck

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	rw, err := reporttar.NewWriter(tw, report.Meta{
		UpboundAccount: c.Account,
		TimeRange:      c.billingPeriod,
		CollectedAt:    time.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "error creating report")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// TODO(branden): Add support for Azure.
	switch {
	case c.Provider == providerGCP:
		if err := reportgcs.GenerateReport(ctx, c.Account, c.Endpoint, c.Bucket, c.billingPeriod, time.Hour, rw); err != nil {
			return err
		}
	case c.Provider == providerAWS:
		if err := reportaws.GenerateReport(ctx, c.Account, c.Endpoint, c.Bucket, c.billingPeriod, rw); err != nil {
			return err
		}
	default:
		return fmt.Errorf(errFmtProviderNotSupported, c.Provider)
	}

	if err := rw.Close(); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

func (c *getCmd) getBillingPeriod() (usage.TimeRange, error) {
	if !c.BillingMonth.IsZero() {
		start := time.Date(c.BillingMonth.Year(), c.BillingMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
		return usage.TimeRange{
			Start: start,
			End:   start.AddDate(0, 1, 0),
		}, nil
	}

	if c.BillingCustom != nil {
		return usage.TimeRange{
			Start: time.Date(
				c.BillingCustom.Start.Year(),
				c.BillingCustom.Start.Month(),
				c.BillingCustom.Start.Day(),
				0,
				0,
				0,
				0,
				time.UTC,
			),
			End: time.Date(
				c.BillingCustom.End.Year(),
				c.BillingCustom.End.Month(),
				c.BillingCustom.End.Day(),
				0,
				0,
				0,
				0,
				time.UTC,
			).AddDate(0, 0, 1),
		}, nil
	}

	return usage.TimeRange{}, fmt.Errorf("billing period is not set")
}

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
