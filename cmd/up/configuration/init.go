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

package configuration

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pterm/pterm"
	"sigs.k8s.io/yaml"

	meta "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

type initCmd struct {
	Name      string `help:"The name of the new configuration package to initialize."`
	Template  string `default:"configuration-template" help:"The template name or URL to use to initialize the new package."`
	Directory string `default:"." help:"The directory to initialize. It must be empty. It will be created if it doesn't exist." type:"path"`
	RefName   string `default:"main" help:"The branch or tag to clone from the template repository." name:"ref-name"`

	Method   string `default:"https" help:"Specify the method to access the repository: 'https' or 'ssh'."`
	SshKey   string `help:"Optional. Specify an SSH key for authentication when initializing the new package. Used when method is 'ssh'."`
	Username string `default:"git" help:"Optional. Specify a username for HTTP(S) authentication. Used when the method is 'https' and an SSH key is not provided."`
	Password string `help:"Optional. Specify a password for HTTP(S) authentication. Used along with the username when the method is 'https'."`
}

// wellKnownTemplates are short aliases for template repositories.
func wellKnownTemplates() map[string]string {
	return map[string]string{
		"configuration-template":     "https://github.com/upbound/configuration-template",
		"configuration-template-ssh": "git@github.com:upbound/configuration-template.git",
	}
}

func (c *initCmd) Run(ctx context.Context, p pterm.TextPrinter) error { // nolint:gocyclo
	// Validation: Ensure that the method is either "ssh" or "https"
	if c.Method != "ssh" && c.Method != "https" {
		return errors.New("invalid method specified; must be either 'ssh' or 'https'")
	}

	// Validation: Ensure that the configuration is valid based on the chosen method.
	if c.Method == "ssh" {
		// SSH URLs provide access to a Git repository via SSH, a secure protocol.
		// To use these URLs, you **must** generate an SSH keypair on your computer and add the public key to your GitHub account.
		if len(c.SshKey) == 0 {
			return errors.New("SSH key must be specified when using SSH method")
		}
		// It's acceptable to have a Password as the passphrase for the SSH key.
	} else if c.Method == "https" {
		if len(c.SshKey) > 0 {
			return errors.New("cannot specify SSH key when using HTTPS method")
		}
	}

	f, err := os.Stat(c.Directory)
	switch {
	case err == nil && !f.IsDir():
		return errors.Errorf("path %s is not a directory", c.Directory)
	case os.IsNotExist(err):
		if err := os.MkdirAll(c.Directory, 0o750); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", c.Directory)
		}
		p.Println("created directory", "path", c.Directory)
	case err != nil:
		return errors.Wrapf(err, "failed to stat directory %s", c.Directory)
	}

	// check the directory only contains allowed files/directories, error out otherwise
	if err := c.checkDirectoryContent(); err != nil {
		return err
	}

	repoURL, ok := wellKnownTemplates()[c.Template]
	if !ok {
		// If the template isn't one of the well-known ones, assume its a URL.
		repoURL = c.Template
	}

	var authMethod interface{} = nil
	if c.Method == "ssh" {
		publicKey, err := ssh.NewPublicKeysFromFile(c.Username, c.SshKey, c.Password)
		if err != nil {
			return errors.Wrapf(err, "failed to create public key from SSH key file")
		}
		authMethod = publicKey
	} else if c.Method == "https" && len(c.Password) > 0 {
		authMethod = &http.BasicAuth{
			Username: c.Username,
			Password: c.Password,
		}
	}

	fs := osfs.New(c.Directory, osfs.WithBoundOS())

	cloneOptions := &git.CloneOptions{
		URL:           repoURL,
		Depth:         1,
		ReferenceName: plumbing.ReferenceName(c.RefName),
	}

	if authMethod != nil {
		switch v := authMethod.(type) {
		case *ssh.PublicKeys:
			cloneOptions.Auth = v
		case *http.BasicAuth:
			cloneOptions.Auth = v
		default:
			return errors.New("unsupported authentication method")
		}
	}

	// Cloning the repository into an in-memory storage, while writing the working tree to the specified filesystem.
	// This allows us to clone the repository without retaining the .git directory on the filesystem, ensuring that
	// the resulting working directory is a clean copy of the repository content without the version control history.
	r, err := git.Clone(memory.NewStorage(), fs, cloneOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to clone repository from %q", repoURL)
	}

	ref, err := r.Head()
	if err != nil {
		return errors.Wrapf(err, "failed to get repository's HEAD from %q", repoURL)
	}

	filePath := filepath.Join(c.Directory, "crossplane.yaml")
	configuration, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return errors.Wrapf(err, "could not read configuration")
	}

	var config meta.Configuration
	err = yaml.Unmarshal(configuration, &config)
	if err != nil {
		return errors.Wrapf(err, "could not parse configuration")
	}

	config.ObjectMeta.Name = c.Name

	modifiedConfiguration, err := yaml.Marshal(&config)
	if err != nil {
		return errors.Wrapf(err, "could not construct configuration")
	}

	err = os.WriteFile(filePath, modifiedConfiguration, 0600)
	if err != nil {
		return errors.Wrapf(err, "could not write configuration")
	}

	p.Printfln("initialized package %q in directory %q from %s (%s)\n",
		c.Name, c.Directory, repoURL, ref.Name().Short())

	return nil
}

func (c *initCmd) checkDirectoryContent() error {
	entries, err := os.ReadDir(c.Directory)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", c.Directory)
	}
	notAllowedEntries := make([]string, 0)
	for _, entry := range entries {
		// .git directory is allowed
		if entry.Name() == ".git" && entry.IsDir() {
			continue
		}
		// add all other entries to the list of unauthorized entries
		notAllowedEntries = append(notAllowedEntries, entry.Name())
	}
	if len(notAllowedEntries) > 0 {
		return errors.Errorf("directory %s is not empty, contains existing files/directories: %s", c.Directory, strings.Join(notAllowedEntries, ", "))
	}
	return nil
}
