package uxp

// Installer can install and manage UXP in a Kubernetes cluster.
// TODO(hasheddan): support custom error types, such as AlreadyExists.
type Installer interface {
	GetCurrentVersion() (string, error)
	Install(version string) error
	Upgrade(version string) error
	Uninstall() error
}
