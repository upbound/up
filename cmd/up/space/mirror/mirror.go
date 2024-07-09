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

package mirror

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/oci"
	"github.com/upbound/up/internal/upterm"
)

type Cmd struct {
	ToDir   string `optional:"" help:"Specify the path to the local directory where images will be exported as .tgz files." short:"t"`
	To      string `optional:"" help:"Specify the destination registry." short:"d"`
	Version string `required:"" help:"Specify the specific Spaces version for which you want to mirror the images." short:"v"`
}

type spinner struct {
	*pterm.SpinnerPrinter
}

func (s spinner) Start(text ...interface{}) (Printer, error) {
	return s.SpinnerPrinter.Start(text...)
}

func (j *uxpVersionsPath) GetSupportedVersions() ([]string, error) {
	return j.Controller.Crossplane.SupportedVersions, nil
}

// Run executes the mirror command.
func (c *Cmd) Run(ctx context.Context, printer upterm.ObjectPrinter) (rErr error) { //nolint:gocyclo

	if c.ToDir != "" {
		info, err := os.Stat(c.ToDir)
		switch {
		case os.IsNotExist(err):
			// Directory does not exist, create it
			if err := os.MkdirAll(c.ToDir, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case err != nil:
			// An error occurred while trying to check the directory
			return fmt.Errorf("failed to check directory: %w", err)
		case !info.IsDir():
			// Path exists but is not a directory
			return fmt.Errorf("path exists but is not a directory")
		}

		// Add trailing slash if needed
		if !strings.HasSuffix(c.ToDir, "/") {
			c.ToDir += "/"
		}
	}

	// remove leading v
	c.Version = strings.TrimPrefix(c.Version, "v")

	// use crane auth from local keychain
	craneOpts := []crane.Option{
		crane.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	for _, repo := range artifacts.oci {
		if err := c.mirrorWithExtraImages(ctx, printer, repo, craneOpts); err != nil {
			return errors.Wrap(err, "mirror artifacts failed")
		}
	}

	for _, image := range artifacts.images {
		if err := c.mirrorArtifact(printer, image, craneOpts); err != nil {
			return fmt.Errorf("mirror artifacts failed: %w", err)
		}
	}

	if !printer.DryRun {
		if len(c.ToDir) > 1 {
			pterm.Println("\nSuccessfully exported artifacts for Spaces!")
		}
		if len(c.To) > 1 {
			pterm.Println("\nSuccessfully mirrored artifacts for Spaces!")
		}
	}

	return nil
}

func (c *Cmd) mirrorWithExtraImages(ctx context.Context, printer upterm.ObjectPrinter, repo repository, craneOpts []crane.Option) error { //nolint:gocyclo
	setupStyling(printer)
	upterm.DefaultObjPrinter.Pretty = true

	if !printer.DryRun {
		if len(c.ToDir) > 1 {
			pterm.Println("Export artifacts for spaces ...")
		}
		if len(c.To) > 1 {
			pterm.Println("Mirroring mirrored artifacts for spaces ...")
		}
	}

	DefaultSpinner = &spinner{upterm.CheckmarkSuccessSpinner}

	s := logAndStartSpinner(printer, "Scanning tags to export...")

	// check if version is available
	tags, err := oci.ListTags(ctx, repo.chart)
	if err != nil {
		return fmt.Errorf("listing tags: %w", err)
	}

	if !oci.TagExists(tags, c.Version) {
		return fmt.Errorf("version %s not found in the list of tags for %s", c.Version, repo.chart)
	}

	if !printer.DryRun {
		s.Success("Scanning tags to export...")
	}
	// mirror main chart
	if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:%s", repo.chart, c.Version), craneOpts); err != nil {
		return fmt.Errorf("mirroring chart: %w", err)
	}

	for _, subChart := range repo.subCharts {
		if subChart.pathNavigator != nil {
			// extract path from values from main chart
			versions, err := oci.GetValuesFromChart(repo.chart, c.Version, subChart.pathNavigator)
			if err != nil {
				return fmt.Errorf("unable to extract: %w", err)
			}
			for _, version := range versions {
				// mirror sub chart
				if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:%s", subChart.chart, version), craneOpts); err != nil {
					return fmt.Errorf("mirroring chart image %s: %w", subChart.chart, err)
				}
				// mirror image for sub chart
				if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:v%s", subChart.image, version), craneOpts); err != nil {
					return fmt.Errorf("mirroring image %s: %w", subChart.image, err)
				}
			}

		}
	}

	for _, image := range repo.images {
		// mirror all other images with same tag than main chart
		if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:v%s", image, c.Version), craneOpts); err != nil {
			return fmt.Errorf("mirroring image %s: %w", image, err)
		}
	}

	return nil
}

func (c *Cmd) mirrorArtifact(printer upterm.ObjectPrinter, artifact string, craneOpts []crane.Option) error {

	path := fmt.Sprintf("%s%s.tgz", c.ToDir, oci.GetArtifactName(artifact))
	rawArtifactName := oci.RemoveDomainAndOrg(artifact)

	if printer.DryRun {
		if len(c.ToDir) > 1 {
			pterm.Printfln("crane pull %s %s", artifact, path)
		}
		if len(c.To) > 1 {
			pterm.Printfln("crane copy %s %s/%s", artifact, c.To, rawArtifactName)
		}
		return nil
	}

	if len(c.ToDir) > 1 {
		img, err := crane.Pull(artifact, craneOpts...)
		if err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
		follow := fmt.Sprintf("save artifact %s ...", artifact)
		s := logAndStartSpinner(printer, follow)
		if err := crane.Save(img, artifact, path); err != nil {
			s.Fail(follow)
			return fmt.Errorf("saving tarball %s: %w", path, err)
		}
		s.Success(follow)
	} else {
		follow := fmt.Sprintf("mirror artifact %s to %s", rawArtifactName, c.To)
		s := logAndStartSpinner(printer, follow)
		if err := crane.Copy(artifact, fmt.Sprintf("%s/%s", c.To, rawArtifactName), craneOpts...); err != nil {
			s.Fail(follow)
			return fmt.Errorf("copy/push failed %s: %w", artifact, err)
		}
		s.Success(follow)
	}
	return nil
}
