package ctx

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	ctpcmd "github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

type State interface {
	Items(ctx context.Context) ([]list.Item, error)
}

type base struct {
	upCtx *upbound.Context
}

type Action interface {
	Exec(ctx context.Context, m model) (model, error)
}

type ActionFunc func(ctx context.Context, m model) (model, error)

func (f ActionFunc) Exec(ctx context.Context, m model) (model, error) {
	return f(ctx, m)
}

type Termination struct {
	Err     error
	Message string
}

type Space struct {
	base
	spaceKubeconfig clientcmd.ClientConfig
}

func (sg *Space) Items(ctx context.Context) ([]list.Item, error) {
	config, err := sg.spaceKubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}
	nss := &corev1.NamespaceList{}
	if err := cl.List(ctx, nss); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(nss.Items)+1)
	//items = append(items, item{text: "..", kind: "profiles"})
	for _, ns := range nss.Items {
		items = append(items, item{text: ns.Name, kind: "group", action: ActionFunc(func(ctx context.Context, m model) (model, error) {
			m.state = &Group{kubeconfig: sg.spaceKubeconfig, name: ns.Name, base: base{upCtx: sg.upCtx}}
			return m, nil
		})})
	}

	if len(nss.Items) == 0 {
		items = append(items, item{text: "No groups found"})
	}

	return items, nil
}

type Group struct {
	base

	kubeconfig clientcmd.ClientConfig
	name       string
}

func (sg *Group) Items(ctx context.Context) ([]list.Item, error) {
	// list controlplanes in group
	config, err := sg.kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}
	ctps := &spacesv1beta1.ControlPlaneList{}
	if err := cl.List(ctx, ctps, client.InNamespace(sg.name)); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(ctps.Items)+2)

	items = append(items, item{text: "Save as kubectl context", action: ActionFunc(func(ctx context.Context, m model) (model, error) {
		raw, err := sg.kubeconfig.RawConfig()
		if err != nil {
			return m, err
		}
		if err := clientcmdapi.MinifyConfig(&raw); err != nil {
			return m, err
		}
		raw.Contexts[raw.CurrentContext].Namespace = sg.name
		if err := kube.MergeIntoKubeConfig(&raw, "", true, kube.VerifyKubeConfig(sg.upCtx.WrapTransport)); err != nil {
			return m, err
		}
		m.termination = &Termination{
			// TODO: print available Upbound kinds
			Message: fmt.Sprintf("Current context set to %s pointing to grouop %q.", raw.CurrentContext, sg.name),
		}
		return m, nil
	}), padding: []int{1, 0}})

	items = append(items, item{text: "..", kind: "groups", action: ActionFunc(func(ctx context.Context, m model) (model, error) {
		m.state = &Space{spaceKubeconfig: sg.kubeconfig, base: base{upCtx: sg.upCtx}}
		return m, nil
	})})

	for _, ctp := range ctps.Items {
		items = append(items, item{text: ctp.Name, kind: "controlplane", action: ActionFunc(func(ctx context.Context, m model) (model, error) {
			m.state = &ControlPlane{ctp: types.NamespacedName{Name: ctp.Name, Namespace: sg.name}, kubeconfig: sg.kubeconfig, base: base{upCtx: sg.upCtx}}
			return m, nil
		})})
	}

	if len(ctps.Items) == 0 {
		items = append(items, item{text: "No ControlPlanes found"})
	}

	return items, nil
}

type ControlPlane struct {
	base

	ctp        types.NamespacedName
	kubeconfig clientcmd.ClientConfig
}

func (sg *ControlPlane) Items(ctx context.Context) ([]list.Item, error) {
	return []list.Item{
		item{text: fmt.Sprintf("Connect to %s", sg.ctp), action: ActionFunc(func(ctx context.Context, m model) (model, error) {
			// get connection secret
			config, err := sg.kubeconfig.ClientConfig()
			if err != nil {
				return m, err
			}
			cl, err := dynamic.NewForConfig(config)
			if err != nil {
				return m, err
			}
			getter := space.New(cl)
			ctpConfig, err := getter.GetKubeConfig(ctx, sg.ctp)
			if err != nil {
				return m, err
			}

			// load user kubeconfig from filesystem.
			userKubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{},
			).RawConfig()
			if err != nil {
				return m, err
			}

			connectContextName := ctpcmd.ConnectControlplaneContextName(sg.upCtx.Account, sg.ctp, userKubeConfig.CurrentContext)
			ctpConfig, err = kubeconfig.ExtractControlPlaneContext(ctpConfig, kubeconfig.ExpectedConnectionSecretContext(sg.upCtx.Account, sg.ctp.Name), connectContextName)
			if err != nil {
				return m, err
			}
			if err := kube.MergeIntoKubeConfig(ctpConfig, "", true, kube.VerifyKubeConfig(sg.upCtx.WrapTransport)); err != nil {
				return m, err
			}
			m.termination = &Termination{
				Message: fmt.Sprintf("Current context set to %s", connectContextName),
			}
			return m, nil
		}), padding: []int{1, 0}},
		item{text: "..", kind: "group", action: ActionFunc(func(ctx context.Context, m model) (model, error) {
			m.state = &Group{kubeconfig: sg.kubeconfig, name: sg.ctp.Namespace, base: base{upCtx: sg.upCtx}}
			return m, nil
		})},
	}, nil
}
