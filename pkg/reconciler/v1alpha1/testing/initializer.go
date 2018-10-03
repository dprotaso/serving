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

package testing

import (
	fakecachingclientset "github.com/knative/caching/pkg/client/clientset/versioned/fake"
	fakesharedclientset "github.com/knative/pkg/client/clientset/versioned/fake"
	fakeclientset "github.com/knative/serving/pkg/client/clientset/versioned/fake"
	k8sinject "github.com/knative/serving/pkg/reconciler/inject"
	"github.com/knative/serving/pkg/reconciler/testing"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/inject"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

var PhaseSetup testing.Initializer = Setup
var ReconcilerSetup testing.Initializer = Setup

func Setup(obj interface{}, objs []runtime.Object) []testing.FakeClient {
	ls := NewListers(objs)

	kubeClient := fakekubeclientset.NewSimpleClientset(ls.GetKubeObjects()...)
	sharedClient := fakesharedclientset.NewSimpleClientset(ls.GetSharedObjects()...)
	servingClient := fakeclientset.NewSimpleClientset(ls.GetServingObjects()...)
	cachingClient := fakecachingclientset.NewSimpleClientset(ls.GetCachingObjects()...)

	k8sinject.KubeClientInto(kubeClient, obj)
	k8sinject.EventRecorderInto(&record.FakeRecorder{}, obj)
	k8sinject.ServiceListerInto(ls.GetK8sServiceLister(), obj)
	k8sinject.DeploymentListerInto(ls.GetDeploymentLister(), obj)
	k8sinject.EndpointsListerInto(ls.GetEndpointsLister(), obj)
	k8sinject.ConfigMapListerInto(ls.GetConfigMapLister(), obj)

	inject.ServingClientInto(servingClient, obj)
	inject.SharedClientInto(sharedClient, obj)
	inject.ServiceListerInto(ls.GetServiceLister(), obj)
	inject.ConfigurationListerInto(ls.GetConfigurationLister(), obj)
	inject.RevisionListerInto(ls.GetRevisionLister(), obj)
	inject.RouteListerInto(ls.GetRouteLister(), obj)
	inject.PodAutoscalerListerInto(ls.GetKPALister(), obj)
	inject.VirtualServiceListerInto(ls.GetVirtualServiceLister(), obj)
	inject.ImageListerInto(ls.GetImageLister(), obj)

	return []testing.FakeClient{
		kubeClient,
		sharedClient,
		servingClient,
		cachingClient,
	}

}
