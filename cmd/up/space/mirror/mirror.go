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
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v3"

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

// Run executes the mirror command.
func (c *Cmd) Run(ctx context.Context, printer upterm.ObjectPrinter) (rErr error) { //nolint:gocyclo
	configData, pathNavigator := initConfig()
	var artifacts config
	err := yaml.Unmarshal(configData, &artifacts)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	for _, repo := range artifacts.OCI {
		for i := range repo.SubResources {
			subChart := &repo.SubResources[i]
			if subChart.PathNavigatorType != "" {
				pathValueType, ok := pathNavigator[subChart.PathNavigatorType]
				if ok {
					pathValue := reflect.New(pathValueType).Interface().(oci.PathNavigator)
					subChart.PathNavigator = pathValue
				}
			}
		}
	}

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

	for _, repo := range artifacts.OCI {
		if err := c.mirrorWithExtraImages(ctx, printer, repo, craneOpts); err != nil {
			return errors.Wrap(err, "mirror artifacts failed")
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

func (c *Cmd) mirrorWithExtraImages(ctx context.Context, printer upterm.ObjectPrinter, repo Repository, craneOpts []crane.Option) error { //nolint:gocyclo
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
	tags, err := oci.ListTags(ctx, repo.Chart)
	if err != nil {
		return fmt.Errorf("listing tags: %w", err)
	}

	if !oci.TagExists(tags, c.Version) {
		return fmt.Errorf("version %s not found in the list of tags for %s", c.Version, repo.Chart)
	}

	if !printer.DryRun {
		s.Success("Scanning tags to export...")
	}
	// mirror main chart
	if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:%s", repo.Chart, c.Version), craneOpts); err != nil {
		return fmt.Errorf("mirroring chart: %w", err)
	}

	for _, subResource := range repo.SubResources {
		if subResource.PathNavigator != nil {
			// extract path from values from main chart
			versions, err := oci.GetValuesFromChart(repo.Chart, c.Version, subResource.PathNavigator)
			if err != nil {
				return fmt.Errorf("unable to extract: %w", err)
			}
			for _, version := range versions {
				// mirror sub resource
				if len(subResource.Chart) > 0 {
					if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:%s", subResource.Chart, version), craneOpts); err != nil {
						return fmt.Errorf("mirroring chart image %s: %w", subResource.Chart, err)
					}
				}
				// mirror image for sub resource
				if len(subResource.Image) > 0 {
					// Check if the version starts with 'v'
					versionWithV := version
					if !strings.HasPrefix(version, "v") {
						versionWithV = "v" + version
					}
					imageWithVersion := fmt.Sprintf("%s:%s", subResource.Image, versionWithV)
					if err := c.mirrorArtifact(printer, imageWithVersion, craneOpts); err != nil {
						return fmt.Errorf("mirroring image %s: %w", subResource.Image, err)
					}
				}
			}
		}
	}

	baseVersion, err := semver.NewVersion(c.Version)
	if err != nil {
		return fmt.Errorf("error parsing space version: %w", err)
	}
	for _, image := range repo.Images {
		include := true
		if image.CompatibleChartVersion != "" {
			constraint, err := semver.NewConstraint(image.CompatibleChartVersion)
			if err != nil {
				continue
			}
			include = constraint.Check(baseVersion) || oci.CheckPreReleaseConstraint(constraint, baseVersion)
		}

		if include {
			version := fmt.Sprintf("v%s", c.Version)
			image := image.Image
			// with static tags
			if parts := strings.Split(image, ":"); len(parts) > 1 {
				image = parts[0]
				version = parts[1]
			}
			if err := c.mirrorArtifact(printer, fmt.Sprintf("%s:%s", image, version), craneOpts); err != nil {
				return fmt.Errorf("mirroring image %s: %w", image, err)
			}
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
