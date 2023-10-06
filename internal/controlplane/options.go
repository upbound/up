package controlplane

type Options struct {
	// Connection Secret Name
	SecretName string
	// Connection Secret Namespace
	SecretNamespace string

	Description string

	ConfigurationName string
}
