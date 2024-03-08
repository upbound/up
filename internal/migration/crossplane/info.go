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

package crossplane

import (
	"context"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/upbound/up/internal/migration/meta/v1alpha1"
)

func CollectInfo(ctx context.Context, appsClient appsv1.DeploymentsGetter) (*v1alpha1.CrossplaneInfo, error) {
	dl, err := appsClient.Deployments("").List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list deployments to find Crossplane deployment")
	}

	xp := v1alpha1.CrossplaneInfo{}
	for _, d := range dl.Items {
		if d.Name == "crossplane" {
			xp.Namespace = d.Namespace
			if d.Labels != nil {
				xp.Version = d.Labels["app.kubernetes.io/version"]
				xp.Distribution = d.Labels["helm.sh/chart"]
			}
			for _, c := range d.Spec.Template.Spec.Containers {
				if c.Name == "crossplane" || c.Name == "universal-crossplane" {
					for _, a := range c.Args {
						if strings.HasPrefix(a, "--enable") {
							xp.FeatureFlags = append(xp.FeatureFlags, a)
						}
					}
					break
				}
			}
			break
		}
	}
	return &xp, nil
}

func CollectPackageInfo(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource) (*[]v1alpha1.PackageInfo, error) {
	packages, err := dynamicClient.Resource(gvr).Namespace("").List(ctx, v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	packageInfos := make([]v1alpha1.PackageInfo, 0, len(packages.Items))
	for _, pkg := range packages.Items {
		spec, _ := pkg.UnstructuredContent()["spec"].(map[string]interface{})
		packageName, _ := spec["package"].(string)

		pp := v1alpha1.PackageInfo{
			Name:    pkg.GetName(),
			Package: packageName,
		}
		packageInfos = append(packageInfos, pp)
	}

	return &packageInfos, nil
}
