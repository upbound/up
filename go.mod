module github.com/upbound/up

go 1.16

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/alecthomas/kong v0.2.16
	github.com/crossplane/crossplane-runtime v0.13.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/addlicense v0.0.0-20210428195630-6d92264d7170
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.6.0
	github.com/upbound/up-sdk-go v0.0.0-20210510154259-67152a64ee01
	golang.org/x/term v0.0.0-20201117132131-f5c789dd3221
	helm.sh/helm/v3 v3.5.4
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/cli-runtime v0.20.4
	k8s.io/client-go v0.20.4
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)
