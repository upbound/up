package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Source is a source for interacting with a Config.
type Source interface {
	GetConfig() (*Config, error)
	UpdateConfig(*Config) error
}

// NewFSSource constructs a new FSSource.
func NewFSSource(modifiers ...FSSourceModifier) (*FSSource, error) {
	src := &FSSource{
		fs:   afero.NewOsFs(),
		home: os.UserHomeDir,
	}
	for _, m := range modifiers {
		m(src)
	}
	h, err := src.home()
	if err != nil {
		return nil, err
	}
	src.dirPath = filepath.Join(h, ConfigDir)
	src.path = filepath.Join(src.dirPath, ConfigFile)
	_, err = src.fs.Stat(src.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := src.fs.MkdirAll(filepath.Join(h, ConfigDir), 0755); err != nil {
			return nil, err
		}
		f, err := src.fs.OpenFile(src.path, os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		defer f.Close() // nolint:errcheck
	}
	return src, nil
}

// FSSourceModifier modifies an FSSource.
type FSSourceModifier func(*FSSource)

// FSSource provides a filesystem source for interacting with a Config.
type FSSource struct {
	fs      afero.Fs
	home    HomeDirFn
	path    string
	dirPath string
}

// HomeDirFn indicates the location of a user's home directory.
type HomeDirFn func() (string, error)

// GetConfig fetches the config from a filesystem.
func (src *FSSource) GetConfig() (*Config, error) {
	f, err := src.fs.Open(src.path)
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint:errcheck
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	conf := &Config{}
	if len(b) == 0 {
		return conf, nil
	}
	if err := json.Unmarshal(b, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// UpdateConfig updates the Config in the filesystem.
func (src *FSSource) UpdateConfig(c *Config) error {
	f, err := src.fs.OpenFile(src.path, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	// NOTE(hasheddan): We both defer and explicitly call Close() to ensure that
	// we close the file in the case that we encounter an error before write,
	// and that we return an error in the case that we write and then fail to
	// close the file (i.e. write buffer is not flushed). In the latter case the
	// deferred Close() will error (see https://golang.org/pkg/os/#File.Close),
	// but we do not check it.
	defer f.Close() // nolint:errcheck
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	return f.Close()
}
