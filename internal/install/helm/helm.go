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

package helm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/afero"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/install"
)

const (
	helmDriverSecret = "secret"
	defaultCacheDir  = ".cache/up/charts"
	defaultNamespace = "upbound-system"
	allVersions      = ">0.0.0-0"
	waitTimeout      = 10 * time.Minute
)

const (
	errGetInstalledReleaseFmt            = "could not identify installed release for %s in namespace %s"
	errGetInstalledReleaseOrAlternateFmt = "could not identify installed release for %s or %s in namespace %s"
	errVerifyInstalledVersion            = "could not identify current version"
	errVerifyChartNotInstalled           = "could not verify that chart is not already installed"
	errChartAlreadyInstalledFmt          = "chart already installed with version %s"
	errPullChart                         = "could not pull chart"
	errGetLatestPulled                   = "could not identify chart pulled as latest"
	errCorruptTempDirFmt                 = "corrupt chart tmp directory, consider removing cache (%s)"
	errMoveLatest                        = "could not move latest pulled chart to cache"

	errUpgradeFromAlternateVersionFmt = "cannot upgrade %s to %s with version mismatch"
	errFailedUpgradeFailedRollback    = "failed upgrade resulted in a failed rollback"
	errFailedUpgradeRollback          = "failed upgrade was rolled back"
)

type helmPuller interface {
	Run(string) (string, error)
	SetDestDir(string)
	SetVersion(string)
}

type puller struct { //nolint:unused
	*action.Pull
}

func (p *puller) SetDestDir(dir string) {
	p.DestDir = dir
}

func (p *puller) SetVersion(version string) {
	p.Version = version
}

type helmGetter interface {
	Run(string) (*release.Release, error)
}

type helmInstaller interface {
	Run(*chart.Chart, map[string]any) (*release.Release, error)
}

type helmUpgrader interface {
	Run(string, *chart.Chart, map[string]any) (*release.Release, error)
}

type helmRollbacker interface {
	Run(string) error
}

type helmUninstaller interface {
	Run(name string) (*release.UninstallReleaseResponse, error)
}

// TempDirFn knows how to create a temporary directory in a filesystem.
type TempDirFn func(afero.Fs, string, string) (string, error)

// LoaderFn knows how to load a helm chart.
type LoaderFn func(name string) (*chart.Chart, error)

// HomeDirFn indicates the location of a user's home directory.
type HomeDirFn func() (string, error)

type installer struct {
	repoURL         *url.URL
	chartFile       *os.File
	chartName       string
	releaseName     string
	alternateChart  string
	namespace       string
	cacheDir        string
	rollbackOnError bool
	force           bool
	wait            bool
	noHooks         bool
	home            HomeDirFn
	fs              afero.Fs
	tempDir         TempDirFn
	log             logging.Logger
	oci             bool

	// Auth
	username string
	password string

	// Clients
	pullClient      helmPuller
	getClient       helmGetter
	installClient   helmInstaller
	upgradeClient   helmUpgrader
	rollbackClient  helmRollbacker
	uninstallClient helmUninstaller

	// Loader
	load LoaderFn
}

// InstallerModifierFn modifies the installer.
type InstallerModifierFn func(*installer)

// WithNamespace sets the namespace for the helm installer.
func WithNamespace(ns string) InstallerModifierFn {
	return func(h *installer) {
		h.namespace = ns
	}
}

// WithAlternateChart sets an alternate chart that is compatible to upgrade from if installed.
func WithAlternateChart(chartName string) InstallerModifierFn {
	return func(h *installer) {
		h.alternateChart = chartName
	}
}

// WithBasicAuth sets the username and password for the helm installer.
func WithBasicAuth(username, password string) InstallerModifierFn {
	return func(h *installer) {
		h.username = username
		h.password = password
	}
}

// IsOCI indicates that the chart is an OCI image.
func IsOCI() InstallerModifierFn {
	return func(h *installer) {
		h.oci = true
	}
}

// WithLogger sets the logger for the helm installer.
func WithLogger(l logging.Logger) InstallerModifierFn {
	return func(h *installer) {
		h.log = l
	}
}

// WithCacheDir sets the cache directory for the helm installer.
func WithCacheDir(c string) InstallerModifierFn {
	return func(h *installer) {
		h.cacheDir = c
	}
}

// WithChart sets the chart to be installed/upgraded
func WithChart(chartFile *os.File) InstallerModifierFn {
	return func(h *installer) {
		h.chartFile = chartFile
	}
}

// RollbackOnError will cause installer to rollback on failed upgrade.
func RollbackOnError(r bool) InstallerModifierFn {
	return func(h *installer) {
		h.rollbackOnError = r
	}
}

// Force will force operations when possible.
func Force(f bool) InstallerModifierFn {
	return func(h *installer) {
		h.force = f
	}
}

// Wait will wait operations till they are completed.
func Wait() InstallerModifierFn {
	return func(h *installer) {
		h.wait = true
	}
}

// WithNoHooks will disable uninstall hooks
func WithNoHooks() InstallerModifierFn {
	return func(h *installer) {
		h.noHooks = true
	}
}

// NewManager builds a helm install manager for UXP.
func NewManager(config *rest.Config, chartName string, repoURL *url.URL, modifiers ...InstallerModifierFn) (install.Manager, error) { // nolint:gocyclo
	h := &installer{
		repoURL:     repoURL,
		chartName:   chartName,
		releaseName: chartName,
		namespace:   defaultNamespace,
		home:        os.UserHomeDir,
		fs:          afero.NewOsFs(),
		tempDir:     afero.TempDir,
		log:         logging.NewNopLogger(),
		load:        loader.Load,
	}
	for _, m := range modifiers {
		m(h)
	}

	if h.cacheDir == "" {
		home, err := h.home()
		if err != nil {
			return nil, err
		}
		h.cacheDir = filepath.Join(home, defaultCacheDir)
	}
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(newRESTClientGetter(config, h.namespace), h.namespace, helmDriverSecret, func(format string, v ...any) {
		h.log.Debug(fmt.Sprintf(format, v))
	}); err != nil {
		return nil, err
	}

	_, err := h.fs.Stat(h.cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := h.fs.MkdirAll(h.cacheDir, 0755); err != nil {
			return nil, err
		}
	}

	// Pull Client
	if h.oci {
		h.pullClient = newRegistryPuller(withRemoteOpts(remote.WithAuth(&authn.Basic{
			Username: h.username,
			Password: h.password,
		})), withRepoURL(h.repoURL))
	} else {
		// TODO(hasheddan): we currently use our own OCI client instead of the
		// upstream Helm support.
		p := action.NewPullWithOpts(action.WithConfig(&action.Configuration{}))
		p.DestDir = h.cacheDir
		p.Username = h.username
		p.Password = h.password
		p.Devel = true
		p.Username = h.username
		p.Password = h.password
		p.Settings = &cli.EnvSettings{}
		p.RepoURL = h.repoURL.String()
		h.pullClient = &puller{p}
	}

	// Get Client
	h.getClient = action.NewGet(actionConfig)

	// Install Client
	ic := action.NewInstall(actionConfig)
	ic.Namespace = h.namespace
	ic.ReleaseName = h.chartName
	ic.Wait = h.wait
	ic.Timeout = waitTimeout
	ic.DisableHooks = h.noHooks
	h.installClient = ic

	// Upgrade Client
	uc := action.NewUpgrade(actionConfig)
	uc.Namespace = h.namespace
	uc.Wait = h.wait
	uc.Timeout = waitTimeout
	uc.DisableHooks = h.noHooks
	h.upgradeClient = uc

	// Uninstall Client
	unc := action.NewUninstall(actionConfig)
	unc.Wait = h.wait
	unc.Timeout = waitTimeout
	unc.DisableHooks = h.noHooks
	h.uninstallClient = unc

	// Rollback Client
	rb := action.NewRollback(actionConfig)
	rb.Wait = h.wait
	rb.Timeout = waitTimeout
	h.rollbackClient = rb

	return h, nil
}

// GetCurrentVersion gets the current UXP version in the cluster.
func (h *installer) GetCurrentVersion() (string, error) {
	var release *release.Release
	var err error
	release, err = h.getClient.Run(h.chartName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return "", err
	}
	if errors.Is(err, driver.ErrReleaseNotFound) {
		if h.alternateChart != "" {
			// TODO(hasheddan): add logging indicating fallback to crossplane.
			if release, err = h.getClient.Run(h.alternateChart); err != nil {
				return "", errors.Wrapf(err, errGetInstalledReleaseOrAlternateFmt, h.chartName, h.alternateChart, h.namespace)
			}
			h.releaseName = h.alternateChart
		} else {
			return "", errors.Wrapf(err, errGetInstalledReleaseFmt, h.chartName, h.namespace)
		}
	}
	if release == nil || release.Chart == nil || release.Chart.Metadata == nil {
		return "", errors.New(errVerifyInstalledVersion)
	}
	return release.Chart.Metadata.Version, nil
}

// Install installs in the cluster.
func (h *installer) Install(version string, parameters map[string]any) error {
	// make sure no version is already installed
	current, err := h.GetCurrentVersion()
	if err == nil {
		return errors.Errorf(errChartAlreadyInstalledFmt, current)
	}
	if !errors.Is(err, driver.ErrReleaseNotFound) {
		return errors.Wrap(err, errVerifyChartNotInstalled)
	}

	var helmChart *chart.Chart
	if h.chartFile == nil {
		// install desired version from repo
		helmChart, err = h.pullAndLoad(version)
	} else {
		// install specified chart from file or folder
		// We assume a uxp or a crossplane chart is referred.
		// For dev purposes, no need to assert this.
		// (see above release check)
		helmChart, err = h.load(h.chartFile.Name())
	}
	if err != nil {
		return err
	}

	_, err = h.installClient.Run(helmChart, parameters)
	return err
}

// Upgrade upgrades an existing installation to a new version.
func (h *installer) Upgrade(version string, parameters map[string]any) error {
	// check if version exists
	current, err := h.GetCurrentVersion()
	if err != nil {
		return err
	}
	if h.releaseName == h.alternateChart && !equivalentVersions(current, version) && !h.force {
		return errors.Errorf(errUpgradeFromAlternateVersionFmt, h.alternateChart, h.chartName)
	}

	var helmChart *chart.Chart
	if h.chartFile == nil {
		helmChart, err = h.pullAndLoad(version)
	} else {
		// upgrade specified chart from file or folder
		// We assume a uxp or a crossplane chart is referred.
		// For dev purposes, no need to assert this.
		// (see above release check)
		helmChart, err = h.load(h.chartFile.Name())
	}
	if err != nil {
		return err
	}

	_, upErr := h.upgradeClient.Run(h.releaseName, helmChart, parameters)
	if upErr != nil && h.rollbackOnError {
		if rErr := h.rollbackClient.Run(h.releaseName); rErr != nil {
			return errors.Wrap(rErr, errFailedUpgradeFailedRollback)
		}
		return errors.Wrap(upErr, errFailedUpgradeRollback)
	}
	return upErr
}

// Uninstall uninstalls an installation.
func (h *installer) Uninstall() error {
	_, err := h.uninstallClient.Run(h.chartName)
	return err
}

// pullAndLoad pulls and loads a chart or fetches it from the cache.
func (h *installer) pullAndLoad(version string) (*chart.Chart, error) { //nolint:gocyclo
	// check to see if version is cached
	if version != "" {
		// helm strips versions with leading v, which can cause issues when fetching
		// the chart from the cache.
		// version = strings.TrimPrefix(version, "v")
		fileName := filepath.Join(h.cacheDir, fmt.Sprintf("%s-%s.tgz", h.chartName, version))
		if _, err := h.fs.Stat(filepath.Join(h.cacheDir, fileName)); err != nil {
			h.pullClient.SetDestDir(h.cacheDir)
			if err := h.pullChart(version); err != nil {
				return nil, errors.Wrap(err, errPullChart)
			}
		}
		return h.load(fileName)
	}
	tmp, err := h.tempDir(h.fs, h.cacheDir, "")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := h.fs.RemoveAll(tmp); err != nil {
			h.log.Debug("failed to clean up temporary directory", "error", err)
		}
	}()
	h.pullClient.SetDestDir(tmp)
	if err := h.pullChart(version); err != nil {
		return nil, errors.Wrap(err, errPullChart)
	}
	files, err := afero.ReadDir(h.fs, tmp)
	if err != nil {
		return nil, errors.Wrap(err, errGetLatestPulled)
	}
	if len(files) != 1 {
		return nil, errors.Errorf(errCorruptTempDirFmt, h.cacheDir)
	}
	// load the chart before copying to cache so that we are able to identify
	// this version in the cache if it is explicitly specified in a future
	// install or upgrade.
	tmpFileName := filepath.Join(tmp, files[0].Name())
	c, err := h.load(tmpFileName)
	if err != nil {
		return nil, err
	}
	fileName := filepath.Join(h.cacheDir, fmt.Sprintf("%s-%s.tgz", h.chartName, c.Metadata.Version))
	if err := h.fs.Rename(tmpFileName, fileName); err != nil {
		return nil, errors.Wrap(err, errMoveLatest)
	}
	return c, nil
}

func (h *installer) pullChart(version string) error {
	// NOTE(hasheddan): Because UXP uses different Helm repos for stable and
	// development versions, we are safe to set version to latest in repo
	// regardless of whether stable or unstable is specified.
	if version == "" {
		version = allVersions
	}
	h.pullClient.SetVersion(version)
	_, err := h.pullClient.Run(h.chartName)
	return err
}

// equivalentVersions determines if two versions are equivalent by comparing
// their major, minor, and patch versions. This is used to determine if a
// crossplane version can be upgraded to the specified universal-crossplane
// version, which should only have what this semver package considers as
// different prerelease data.
func equivalentVersions(current, target string) bool {
	curV, err := semver.NewVersion(current)
	if err != nil {
		return false
	}
	tarV, err := semver.NewVersion(target)
	if err != nil {
		return false
	}
	if curV.Major() == tarV.Major() && curV.Minor() == tarV.Minor() && curV.Patch() == tarV.Patch() {
		return true
	}
	return false
}
