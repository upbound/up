module github.com/upbound/up

go 1.16

require (
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/alecthomas/kong v0.2.16
	github.com/crossplane/crossplane v1.2.2
	github.com/crossplane/crossplane-runtime v0.14.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/addlicense v0.0.0-20210428195630-6d92264d7170
	github.com/google/go-cmp v0.5.5
	github.com/google/go-containerregistry v0.5.1
	github.com/google/uuid v1.2.0
	github.com/goreleaser/nfpm/v2 v2.5.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.1.3
	github.com/upbound/up-sdk-go v0.1.0
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1
	helm.sh/helm/v3 v3.5.4
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/cli-runtime v0.20.4
	k8s.io/client-go v0.20.4
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)
