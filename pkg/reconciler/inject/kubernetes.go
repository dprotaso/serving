/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inject

import (
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type (
	kubeClient interface {
		InjectKubeClientset(kubeclientset.Interface)
	}

	serviceLister interface {
		InjectServiceLister(corev1listers.ServiceLister)
	}

	endpointsLister interface {
		InjectEndpointsLister(corev1listers.EndpointsLister)
	}

	configMapLister interface {
		InjectConfigMapLister(corev1listers.ConfigMapLister)
	}

	deploymentLister interface {
		InjectDeploymentLister(appsv1listers.DeploymentLister)
	}
)

func KubeClientInto(c kubeclientset.Interface, i interface{}) {
	if o, ok := i.(kubeClient); ok {
		o.InjectKubeClientset(c)
	}
}

func ServiceListerInto(s corev1listers.ServiceLister, i interface{}) {
	if o, ok := i.(serviceLister); ok {
		o.InjectServiceLister(s)
	}
}

func EndpointsListerInto(s corev1listers.EndpointsLister, i interface{}) {
	if o, ok := i.(endpointsLister); ok {
		o.InjectEndpointsLister(s)
	}
}

func ConfigMapListerInto(s corev1listers.ConfigMapLister, i interface{}) {
	if o, ok := i.(configMapLister); ok {
		o.InjectConfigMapLister(s)
	}
}

func DeploymentListerInto(s appsv1listers.DeploymentLister, i interface{}) {
	if o, ok := i.(deploymentLister); ok {
		o.InjectDeploymentLister(s)
	}
}

func KubernetesListersInto(inf kubeinformers.SharedInformerFactory, i interface{}) {
	if o, ok := i.(serviceLister); ok {
		o.InjectServiceLister(inf.Core().V1().Services().Lister())
	}

	if o, ok := i.(endpointsLister); ok {
		o.InjectEndpointsLister(inf.Core().V1().Endpoints().Lister())
	}

	if o, ok := i.(deploymentLister); ok {
		o.InjectDeploymentLister(inf.Apps().V1().Deployments().Lister())
	}

	if o, ok := i.(configMapLister); ok {
		o.InjectConfigMapLister(inf.Core().V1().ConfigMaps().Lister())
	}
}
