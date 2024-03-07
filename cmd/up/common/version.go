// Copyright 2023 Upbound Inc
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

// Package common contains common functions
package common

import (
	"context"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errKubeConfig         = "failed to get kubeconfig"
	errCreateK8sClientset = "could not create the clientset for Kubernetes"
	errFetchDeployment    = "could not fetch deployments"
)

// FetchCrossplaneVersion initializes a Kubernetes client and fetches
// and returns the version of the Crossplane deployment. If the version
// does not have a leading 'v', it prepends it.
func FetchCrossplaneVersion() (string, error) {
	var version string
	config, err := ctrl.GetConfig()
	if err != nil {
		return "", errors.Wrap(err, errKubeConfig)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", errors.Wrap(err, errCreateK8sClientset)
	}

	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), v1.ListOptions{
		LabelSelector: "app=crossplane",
	})
	if err != nil {
		return "", errors.Wrap(err, errFetchDeployment)
	}

	for _, deployment := range deployments.Items {
		v, ok := deployment.Labels["app.kubernetes.io/version"]
		if ok {
			if !strings.HasPrefix(v, "v") {
				version = "v" + v
			}
			return version, nil
		}

		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			image := deployment.Spec.Template.Spec.Containers[0].Image
			parts := strings.Split(image, ":")
			if len(parts) > 1 {
				imageTag := parts[1]
				if !strings.HasPrefix(imageTag, "v") {
					imageTag = "v" + imageTag
				}
				return imageTag, nil
			}
		}
	}

	return "", errors.New("Crossplane version or image tag not found")
}

// FetchSpacesVersion initializes a Kubernetes client and fetches
// and returns the version of the spaces-controller deployment.
func FetchSpacesVersion() (string, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return "", errors.Wrap(err, errKubeConfig)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", errors.Wrap(err, errCreateK8sClientset)
	}

	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), v1.ListOptions{
		LabelSelector: "app=spaces-controller",
	})
	if err != nil {
		return "", errors.Wrap(err, errFetchDeployment)
	}

	for _, deployment := range deployments.Items {
		v, ok := deployment.Labels["app.kubernetes.io/version"]
		if ok {
			return v, nil
		}
	}

	return "", errors.New("spaces-controller version not found")
}
