package helm

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type mockGetClient struct {
	runFn func(string) (*release.Release, error)
}

// Run calls the underlying run function.
func (m *mockGetClient) Run(version string) (*release.Release, error) {
	return m.runFn(version)
}

type mockPullClient struct {
	runFn func(string) (string, error)
}

// Run calls the underlying run function.
func (m *mockPullClient) Run(version string) (string, error) {
	return m.runFn(version)
}

// SetDestDir is a no op.
func (m *mockPullClient) SetDestDir(string) {}

// SetVersion is a no op.
func (m *mockPullClient) SetVersion(string) {}

type mockInstallClient struct {
	runFn func(*chart.Chart, map[string]interface{}) (*release.Release, error)
}

// Run calls the underlying run function.
func (m *mockInstallClient) Run(c *chart.Chart, v map[string]interface{}) (*release.Release, error) {
	return m.runFn(c, v)
}

type mockUpgradeClient struct {
	runFn func(string, *chart.Chart, map[string]interface{}) (*release.Release, error)
}

// Run calls the underlying run function.
func (m *mockUpgradeClient) Run(r string, c *chart.Chart, v map[string]interface{}) (*release.Release, error) {
	return m.runFn(r, c, v)
}

type mockRollbackClient struct {
	runFn func(string) error
}

// Run calls the underlying run function.
func (m *mockRollbackClient) Run(r string) error {
	return m.runFn(r)
}

type mockUninstallClient struct {
	runFn func(string) (*release.UninstallReleaseResponse, error)
}

// Run calls the underlying run function.
func (m *mockUninstallClient) Run(r string) (*release.UninstallReleaseResponse, error) {
	return m.runFn(r)
}

func TestGetCurrentVersion(t *testing.T) {
	errBoom := errors.New("boom")
	cases := map[string]struct {
		reason    string
		installer *installer
		version   string
		err       error
	}{
		"ErrorGetRelease": {
			reason: "If unable to get release an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, errBoom
					},
				},
			},
			err: errBoom,
		},
		"ErrorExtractVersion": {
			reason: "If unable to extract version from current release and error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{}, nil
					},
				},
			},
			err: errors.New(errVerifyInstalledVersion),
		},
		"Successful": {
			reason: "If successful version and no error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "a-version",
								},
							},
						}, nil
					},
				},
			},
			version: "a-version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v, err := tc.installer.GetCurrentVersion()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetCurrentVersion(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.version, v); diff != "" {
				t.Errorf("\n%s\nGetCurrentVersion(...): -want error, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	errBoom := errors.New("boom")
	cases := map[string]struct {
		reason    string
		installer *installer
		fsSetup   func() afero.Fs
		version   string
		err       error
	}{
		"ErrorCouldNotVerifyNotInstalled": {
			reason: "If unable to verify that the chart is not already installed an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, errBoom
					},
				},
			},
			fsSetup: afero.NewMemMapFs,
			err:     errors.Wrap(errBoom, errVerifyChartNotInstalled),
		},
		"ErrorPullNewVersion": {
			reason: "If unable to pull specified version an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, driver.ErrReleaseNotFound
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", errBoom
					},
				},
			},
			fsSetup: afero.NewMemMapFs,
			version: "real-version",
			err:     errors.Wrap(errBoom, errPullChart),
		},
		"ErrorInstall": {
			reason: "If unable to install specified version an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, driver.ErrReleaseNotFound
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				installClient: &mockInstallClient{
					runFn: func(*chart.Chart, map[string]interface{}) (*release.Release, error) {
						return nil, errBoom
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return nil, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("test-real-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "real-version",
			err:     errBoom,
		},
		"Successful": {
			reason: "Successful installation should not return an error.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, driver.ErrReleaseNotFound
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				installClient: &mockInstallClient{
					runFn: func(*chart.Chart, map[string]interface{}) (*release.Release, error) {
						return nil, nil
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return nil, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("test-real-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "real-version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.installer.fs = tc.fsSetup()
			err := tc.installer.Install(tc.version)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpgrade(t *testing.T) {
	errBoom := errors.New("boom")
	cases := map[string]struct {
		reason    string
		installer *installer
		fsSetup   func() afero.Fs
		version   string
		err       error
	}{
		"ErrorNotInstalled": {
			reason: "If unable to verify that the chart is installed an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return nil, errBoom
					},
				},
			},
			fsSetup: afero.NewMemMapFs,
			err:     errBoom,
		},
		"ErrorInstalledSameVersion": {
			reason: "If installed version matches upgrade version an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "a-version",
								},
							},
						}, nil
					},
				},
			},
			version: "a-version",
			fsSetup: afero.NewMemMapFs,
			err:     errors.New(errUpgradeVersionsSame),
		},
		"ErrorPullNewVersion": {
			reason: "If unable to pull specified version an error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "diff-version",
								},
							},
						}, nil
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", errBoom
					},
				},
			},
			fsSetup: afero.NewMemMapFs,
			version: "real-version",
			err:     errors.Wrap(errBoom, errPullChart),
		},
		"ErrorUpgradeRollbackSuccessful": {
			reason: "If upgrade fails but rollback is successful, only upgrade error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "a-version",
								},
							},
						}, nil
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				upgradeClient: &mockUpgradeClient{
					runFn: func(string, *chart.Chart, map[string]interface{}) (*release.Release, error) {
						return nil, errBoom
					},
				},
				rollbackClient: &mockRollbackClient{
					runFn: func(string) error {
						return nil
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return nil, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("test-real-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "real-version",
			err:     errBoom,
		},
		"ErrorUpgradeErrorRollback": {
			reason: "If upgrade and rollback fails a wrapped error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "a-version",
								},
							},
						}, nil
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				upgradeClient: &mockUpgradeClient{
					runFn: func(string, *chart.Chart, map[string]interface{}) (*release.Release, error) {
						return nil, errBoom
					},
				},
				rollbackClient: &mockRollbackClient{
					runFn: func(string) error {
						return errBoom
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return nil, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("test-real-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "real-version",
			err:     errors.Wrap(errBoom, errFailedUpgradeRollback),
		},
		"Successful": {
			reason: "If upgrade is successful no error should be returned.",
			installer: &installer{
				getClient: &mockGetClient{
					runFn: func(string) (*release.Release, error) {
						return &release.Release{
							Chart: &chart.Chart{
								Metadata: &chart.Metadata{
									Version: "a-version",
								},
							},
						}, nil
					},
				},
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				upgradeClient: &mockUpgradeClient{
					runFn: func(string, *chart.Chart, map[string]interface{}) (*release.Release, error) {
						return nil, nil
					},
				},
				rollbackClient: &mockRollbackClient{
					runFn: func(string) error {
						return nil
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return nil, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("test-real-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "real-version",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.installer.fs = tc.fsSetup()
			err := tc.installer.Upgrade(tc.version)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpgrade(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPullAndLoad(t *testing.T) {
	errBoom := errors.New("boom")
	cases := map[string]struct {
		reason    string
		installer *installer
		fsSetup   func() afero.Fs
		version   string
		err       error
		want      *chart.Chart
	}{
		"ErrorPullLatestTempDir": {
			reason: "Should return error if pulling latest and unable to create temporary directory.",
			installer: &installer{
				tempDir: func(afero.Fs, string, string) (string, error) {
					return "", errBoom
				},
				cacheDir:  "/",
				chartName: "test",
			},
			fsSetup: afero.NewMemMapFs,
			err:     errBoom,
		},
		"ErrorPullLatest": {
			reason: "Should return error if fail to pull latest.",
			installer: &installer{
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", errBoom
					},
				},
				tempDir: func(afero.Fs, string, string) (string, error) {
					return "", nil
				},
				cacheDir:  "/",
				chartName: "test",
			},
			fsSetup: afero.NewMemMapFs,
			err:     errors.Wrap(errBoom, errPullChart),
		},
		"ErrorPullLatestCorrupt": {
			reason: "Should return error if pulling latest and temporary directory is corrupt.",
			installer: &installer{
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				tempDir: func(afero.Fs, string, string) (string, error) {
					return "/tmp", nil
				},
				cacheDir:  "/",
				chartName: "test",
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("/tmp", 0755)
				f, _ := fs.Create("/tmp/test-a-version.tgz")
				_ = f.Close()
				f, _ = fs.Create("/tmp/test-b-version.tgz")
				_ = f.Close()
				return fs
			},
			err: errors.Errorf(errCorruptTempDirFmt, "/"),
		},
		"SuccessfulPullLatest": {
			reason: "If able to successfully pull and load latest no error should be returned.",
			installer: &installer{
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				tempDir: func(afero.Fs, string, string) (string, error) {
					return "/tmp", nil
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return &chart.Chart{
						Metadata: &chart.Metadata{
							Version: "a-version",
						},
					}, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("/tmp", 0755)
				f, _ := fs.Create("/tmp/test-a-version.tgz")
				_ = f.Close()
				return fs
			},
			want: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "a-version",
				},
			},
		},
		"SuccessfulPullVersion": {
			reason: "If able to successfully pull and load version no error should be returned.",
			installer: &installer{
				pullClient: &mockPullClient{
					runFn: func(string) (string, error) {
						return "", nil
					},
				},
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return &chart.Chart{
						Metadata: &chart.Metadata{
							Version: "a-version",
						},
					}, nil
				},
			},
			fsSetup: afero.NewMemMapFs,
			version: "a-version",
			want: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "a-version",
				},
			},
		},
		"SuccessfulPullCached": {
			reason: "If able to successfully load chart from cache no error should be returned.",
			installer: &installer{
				cacheDir:  "/",
				chartName: "test",
				load: func(string) (*chart.Chart, error) {
					return &chart.Chart{
						Metadata: &chart.Metadata{
							Version: "a-version",
						},
					}, nil
				},
			},
			fsSetup: func() afero.Fs {
				fs := afero.NewMemMapFs()
				f, _ := fs.Create("/test-a-version.tgz")
				_ = f.Close()
				return fs
			},
			version: "a-version",
			want: &chart.Chart{
				Metadata: &chart.Metadata{
					Version: "a-version",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.installer.fs = tc.fsSetup()
			c, err := tc.installer.pullAndLoad(tc.version)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npullAndLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, c, cmpopts.IgnoreUnexported(chart.Chart{})); diff != "" {
				t.Errorf("\n%s\npullAndLoad(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUninstall(t *testing.T) {
	errBoom := errors.New("boom")
	cases := map[string]struct {
		reason    string
		installer *installer
		err       error
	}{
		"Error": {
			reason: "Should return error if uninstall fails.",
			installer: &installer{
				uninstallClient: &mockUninstallClient{
					runFn: func(string) (*release.UninstallReleaseResponse, error) {
						return nil, errBoom
					},
				},
			},
			err: errBoom,
		},
		"Successful": {
			reason: "Should not return error if uninstall is successful.",
			installer: &installer{
				uninstallClient: &mockUninstallClient{
					runFn: func(string) (*release.UninstallReleaseResponse, error) {
						return nil, nil
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.installer.Uninstall(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUninstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
