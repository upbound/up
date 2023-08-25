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

	"cloud.google.com/go/storage"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	gcpopt "google.golang.org/api/option"

	usageaws "github.com/upbound/up/internal/usage/aws"
	"github.com/upbound/up/internal/usage/azure"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/gcp"
	"github.com/upbound/up/internal/usage/report"
	reporttar "github.com/upbound/up/internal/usage/report/file/tar"
	usagetime "github.com/upbound/up/internal/usage/time"
)

const (
	providerAWS   = "aws"
	providerGCP   = "gcp"
	providerAzure = "azure"

	errFmtProviderNotSupported = "%q is not supported"
)

type dateRange usagetime.Range

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
	switch p {
	case providerGCP:
		return nil
	case providerAWS:
		return nil
	case providerAzure:
		return nil
	default:
		return fmt.Errorf(errFmtProviderNotSupported, p)
	}
}

type exportCmd struct {
	Out string `optional:"" short:"o" env:"UP_BILLING_OUT" default:"upbound_billing_report.tgz" help:"Name of the output file."`

	// TODO(branden): Make storage params optional and fetch missing values from spaces cluster.
	Provider            provider `required:"" enum:"aws,gcp,azure," env:"UP_BILLING_PROVIDER" group:"Storage" help:"Storage provider. Must be one of: aws, gcp, azure."`
	Bucket              string   `required:"" env:"UP_BILLING_BUCKET" group:"Storage" help:"Storage bucket."`
	Endpoint            string   `env:"UP_BILLING_ENDPOINT" group:"Storage" help:"Custom storage endpoint."`
	Account             string   `required:"" env:"UP_BILLING_ACCOUNT" group:"Storage" help:"Name of the Upbound account whose billing report is being collected."`
	AzureStorageAccount string   `optional:"" env:"UP_AZURE_STORAGE_ACCOUNT" group:"Storage" help:"Name of the Azure storage account. Required for --provider=azure."`

	BillingMonth    time.Time  `format:"2006-01" required:"" xor:"billingperiod" env:"UP_BILLING_MONTH" group:"Billing period" help:"Export a report for a billing period of one calendar month. Format: 2006-01."`
	BillingCustom   *dateRange `required:"" xor:"billingperiod" env:"UP_BILLING_CUSTOM" group:"Billing period" help:"Export a report for a custom billing period. Date range is inclusive. Format: 2006-01-02/2006-01-02."`
	ForceIncomplete bool       `env:"UP_BILLING_FORCE_INCOMPLETE" group:"Billing period" help:"Export a report for an incomplete billing period."`

	outAbs        string
	billingPeriod usagetime.Range
}

//go:embed export_help.txt
var exportCmdHelp string

func (c *exportCmd) Help() string {
	return exportCmdHelp
}

func (c *exportCmd) Validate() error {
	if c.Provider == providerAzure {
		if c.AzureStorageAccount == "" {
			return fmt.Errorf("--azure-storage-account must be set for --provider=azure")
		}
		if c.Endpoint != "" {
			return fmt.Errorf("--endpoint is not supported for --provider=azure")
		}
	}

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

func (c *exportCmd) Run() error {
	fmt.Printf(
		"Exporting billing report for Upbound account %s from %s to %s.\n",
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

func (c *exportCmd) cleanupOnError() {
	if err := os.Remove(c.outAbs); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up: %s", err)
	}
}

func (c *exportCmd) collectReport() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Make event window iterator.
	window := time.Hour
	var iter event.WindowIterator
	var err error
	switch c.Provider {
	case providerGCP:
		iter, err = c.getGCPIter(ctx, window)
	case providerAWS:
		iter, err = c.getAWSIter(window)
	case providerAzure:
		iter, err = c.getAzureIter(window)
	default:
		return fmt.Errorf(errFmtProviderNotSupported, c.Provider)
	}
	if err != nil {
		return err
	}

	// Make report writer.
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

	// Write report.
	if err := report.MaxResourceCountPerGVKPerMCP(ctx, iter, rw); err != nil {
		return err
	}
	if err := rw.Close(); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

func (c *exportCmd) getGCPIter(ctx context.Context, window time.Duration) (event.WindowIterator, error) {
	opts := []gcpopt.ClientOption{}
	if c.Endpoint != "" {
		opts = append(opts, gcpopt.WithEndpoint(c.Endpoint))
	}
	gcsCli, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "error creating storage client")
	}
	bkt := gcsCli.Bucket(c.Bucket)
	return gcp.NewWindowIterator(bkt, c.Account, c.billingPeriod, window)
}

func (c *exportCmd) getAWSIter(window time.Duration) (event.WindowIterator, error) {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating aws session")
	}
	config := &aws.Config{}
	if c.Endpoint != "" {
		config = &aws.Config{
			Endpoint: aws.String(c.Endpoint),
		}
	}
	s3client := s3.New(sess, config)
	return usageaws.NewWindowIterator(s3client, c.Bucket, c.Account, c.billingPeriod, window)
}

func (c *exportCmd) getAzureIter(window time.Duration) (event.WindowIterator, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	cli, err := azblob.NewClient(fmt.Sprintf("https://%s.blob.core.windows.net/", c.AzureStorageAccount), cred, nil)
	if err != nil {
		return nil, err
	}
	containerCli := cli.ServiceClient().NewContainerClient(c.Bucket)
	return azure.NewWindowIterator(containerCli, c.Account, c.billingPeriod, window)
}

func (c *exportCmd) getBillingPeriod() (usagetime.Range, error) {
	if !c.BillingMonth.IsZero() {
		start := time.Date(c.BillingMonth.Year(), c.BillingMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
		return usagetime.Range{
			Start: start,
			End:   start.AddDate(0, 1, 0),
		}, nil
	}

	if c.BillingCustom != nil {
		return usagetime.Range{
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

	return usagetime.Range{}, fmt.Errorf("billing period is not set")
}

func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
