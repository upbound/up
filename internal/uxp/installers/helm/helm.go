package helm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/uxp"
)

const (
	helmDriverSecret       = "secret"
	defaultCacheDir        = ".cache/up/charts"
	defaultNamespace       = "upbound-system"
	defaultRepoURL         = "https://charts.upbound.io/stable"
	defaultUnstableRepoURL = "https://charts.upbound.io/main"
	defaultChartName       = "universal-crossplane"
)

const (
	errVerifyInstalledVersion   = "could not identify current version"
	errVerifyChartNotInstalled  = "could not verify that chart is not already installed"
	errChartAlreadyInstalledFmt = "chart already installed with version %s"
	errPullChart                = "could not pull chart"
	errGetLatestPulled          = "could not identify chart pulled as latest"
	errCorruptTempDirFmt        = "corrupt chart tmp directory, consider removing cache (%s)"
	errMoveLatest               = "could not move latest pulled chart to cache"

	errUpgradeVersionsSame         = "upgrade version is same as existing"
	errFailedUpgradeFailedRollback = "failed upgrade resulted in a failed rollback"
	errFailedUpgradeRollback       = "failed upgrade was rolled back"
)

type helmPuller interface {
	Run(string) (string, error)
	SetDestDir(string)
	SetVersion(string)
}

type puller struct {
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
	Run(*chart.Chart, map[string]interface{}) (*release.Release, error)
}

type helmUpgrader interface {
	Run(string, *chart.Chart, map[string]interface{}) (*release.Release, error)
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
	chartName       string
	namespace       string
	cacheDir        string
	unstable        bool
	rollbackOnError bool
	home            HomeDirFn
	fs              afero.Fs
	tempDir         TempDirFn
	log             logging.Logger

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

// WithRepoURL sets the repo URL for the helm installer.
func WithRepoURL(u *url.URL) InstallerModifierFn {
	return func(h *installer) {
		h.repoURL = u
	}
}

// WithChartName sets the chart name for the helm installer.
func WithChartName(name string) InstallerModifierFn {
	return func(h *installer) {
		h.chartName = name
	}
}

// WithNamespace sets the namespace for the helm installer.
func WithNamespace(ns string) InstallerModifierFn {
	return func(h *installer) {
		h.namespace = ns
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

// AllowUnstableVersions allows installing development versions using the helm
// installer.
func AllowUnstableVersions(d bool) InstallerModifierFn {
	return func(h *installer) {
		h.unstable = d
	}
}

// RollbackOnError will cause installer to rollback on failed upgrade.
func RollbackOnError(r bool) InstallerModifierFn {
	return func(h *installer) {
		h.rollbackOnError = r
	}
}

// NewInstaller builds a helm installer for UXP.
func NewInstaller(config *rest.Config, modifiers ...InstallerModifierFn) (uxp.Installer, error) { // nolint:gocyclo
	u, err := url.Parse(defaultRepoURL)
	if err != nil {
		return nil, err
	}
	h := &installer{
		repoURL:   u,
		chartName: defaultChartName,
		namespace: defaultNamespace,
		home:      os.UserHomeDir,
		unstable:  false,
		fs:        afero.NewOsFs(),
		tempDir:   afero.TempDir,
		log:       logging.NewNopLogger(),
		load:      loader.Load,
	}
	for _, m := range modifiers {
		m(h)
	}

	// Use default unstable URL if URL is default and unstable is specified.
	if h.unstable && h.repoURL == u {
		unstableURL, err := url.Parse(defaultUnstableRepoURL)
		if err != nil {
			return nil, err
		}
		h.repoURL = unstableURL
	}

	if h.cacheDir == "" {
		home, err := h.home()
		if err != nil {
			return nil, err
		}
		h.cacheDir = filepath.Join(home, defaultCacheDir)
	}
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(newRESTClientGetter(config, h.namespace), h.namespace, helmDriverSecret, func(format string, v ...interface{}) {
		h.log.Debug(fmt.Sprintf(format, v))
	}); err != nil {
		return nil, err
	}

	_, err = h.fs.Stat(h.cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := h.fs.MkdirAll(h.cacheDir, 0755); err != nil {
			return nil, err
		}
	}

	// Pull Client
	p := action.NewPull()
	p.DestDir = h.cacheDir
	p.Devel = h.unstable
	p.Settings = &cli.EnvSettings{}
	p.RepoURL = h.repoURL.String()
	h.pullClient = &puller{p}

	// Get Client
	h.getClient = action.NewGet(actionConfig)

	// Install Client
	ic := action.NewInstall(actionConfig)
	ic.Namespace = h.namespace
	ic.ReleaseName = h.chartName
	h.installClient = ic

	// Upgrade Client
	uc := action.NewUpgrade(actionConfig)
	uc.Namespace = h.namespace
	h.upgradeClient = uc

	// Uninstall Client
	h.uninstallClient = action.NewUninstall(actionConfig)

	// Rollback Client
	rb := action.NewRollback(actionConfig)
	h.rollbackClient = rb

	return h, nil
}

// GetCurrentVersion gets the current UXP version in the cluster.
func (h *installer) GetCurrentVersion() (string, error) {
	release, err := h.getClient.Run(defaultChartName)
	if err != nil {
		return "", err
	}
	if release == nil || release.Chart == nil || release.Chart.Metadata == nil {
		return "", errors.New(errVerifyInstalledVersion)
	}
	return release.Chart.Metadata.Version, nil
}

// Install installs UXP in the cluster.
func (h *installer) Install(version string) error {
	// make sure no version is already installed
	current, err := h.GetCurrentVersion()
	if err == nil {
		return errors.Errorf(errChartAlreadyInstalledFmt, current)
	}
	if !errors.Is(err, driver.ErrReleaseNotFound) {
		return errors.Wrap(err, errVerifyChartNotInstalled)
	}
	// install desired version
	chart, err := h.pullAndLoad(version)
	if err != nil {
		return err
	}
	_, err = h.installClient.Run(chart, map[string]interface{}{})
	return err
}

// Upgrade upgrades an existing UXP installation to a new version.
func (h *installer) Upgrade(version string) error {
	// check if version exists and is not the same as desired
	current, err := h.GetCurrentVersion()
	if err != nil {
		return err
	}
	if version != "" && version == current {
		return errors.New(errUpgradeVersionsSame)
	}
	chart, err := h.pullAndLoad(version)
	if err != nil {
		return err
	}
	_, upErr := h.upgradeClient.Run(h.chartName, chart, map[string]interface{}{})
	if upErr != nil && h.rollbackOnError {
		if rErr := h.rollbackClient.Run(h.chartName); rErr != nil {
			return errors.Wrap(rErr, errFailedUpgradeFailedRollback)
		}
		return errors.Wrap(upErr, errFailedUpgradeRollback)
	}
	return upErr
}

// Uninstall uninstalls a UXP installation.
func (h *installer) Uninstall() error {
	_, err := h.uninstallClient.Run(h.chartName)
	return err
}

// pullAndLoad pulls and loads a chart or fetches it from the catch.
func (h *installer) pullAndLoad(version string) (*chart.Chart, error) {
	// check to see if version is cached
	fileName := filepath.Join(h.cacheDir, fmt.Sprintf("%s-%s.tgz", h.chartName, version))
	if version != "" {
		if _, err := h.fs.Stat(filepath.Join(h.cacheDir, fileName)); err != nil {
			h.pullClient.SetDestDir(h.cacheDir)
			if err := h.pullChart(version); err != nil {
				return nil, errors.Wrap(err, errPullChart)
			}
		}
	} else {
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
		tmpFileName := filepath.Join(tmp, files[0].Name())
		if err := h.fs.Rename(tmpFileName, fileName); err != nil {
			return nil, errors.Wrap(err, errMoveLatest)
		}
	}
	return h.load(fileName)
}

func (h *installer) pullChart(version string) error {
	h.pullClient.SetVersion(version)
	_, err := h.pullClient.Run(h.chartName)
	return err
}
