module github.com/upbound/up

go 1.16

require (
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/alecthomas/kong v0.2.17
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/crossplane/crossplane v1.5.0
	github.com/crossplane/crossplane-runtime v0.15.1-0.20210930095326-d5661210733b
	github.com/docker/docker v20.10.7+incompatible
	github.com/goccy/go-yaml v1.9.4
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/google/addlicense v0.0.0-20210428195630-6d92264d7170
	github.com/google/go-cmp v0.5.6
	github.com/google/go-containerregistry v0.6.0
	github.com/google/uuid v1.2.0
	github.com/goreleaser/nfpm/v2 v2.5.1
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sourcegraph/go-lsp v0.0.0-20200429204803-219e11d77f5d
	github.com/sourcegraph/jsonrpc2 v0.1.0
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.2.1
	github.com/upbound/up-sdk-go v0.1.0
	go.starlark.net v0.0.0-20211013185944-b0039bd2cfe3 // indirect
	golang.org/x/sys v0.0.0-20211103184734-ae416a5f93c7 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	helm.sh/helm/v3 v3.7.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/cli-runtime v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	k8s.io/kubectl v0.22.1
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/crossplane/crossplane-runtime => ../../crossplane/crossplane-runtime
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	k8s.io/kube-openapi => ../../../k8s.io/kube-openapi
)
