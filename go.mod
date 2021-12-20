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
	github.com/fatih/color v1.13.0 // indirect
	github.com/goccy/go-yaml v1.9.5-0.20211210133106-251b4db627e0
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/golang/tools v0.1.7
	github.com/google/addlicense v1.0.0
	github.com/google/go-cmp v0.5.6
	github.com/google/go-containerregistry v0.6.0
	github.com/google/uuid v1.2.0
	github.com/goreleaser/nfpm/v2 v2.5.1
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sourcegraph/go-lsp v0.0.0-20200429204803-219e11d77f5d
	github.com/sourcegraph/jsonrpc2 v0.1.0
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.2.1
	github.com/upbound/up-sdk-go v0.1.0
	go.starlark.net v0.0.0-20211013185944-b0039bd2cfe3 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	helm.sh/helm/v3 v3.7.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/cli-runtime v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	k8s.io/kubectl v0.22.1
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/golang/tools => ./internal/vendor/golang.org/x/tools
)
