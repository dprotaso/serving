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
	cachingclientset "github.com/knative/caching/pkg/client/clientset/versioned"
	cachinginformers "github.com/knative/caching/pkg/client/informers/externalversions"
	cachinglisters "github.com/knative/caching/pkg/client/listers/caching/v1alpha1"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
	istiolisters "github.com/knative/pkg/client/listers/istio/v1alpha3"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"
	kpalisters "github.com/knative/serving/pkg/client/listers/autoscaling/v1alpha1"
	servinglisters "github.com/knative/serving/pkg/client/listers/serving/v1alpha1"
)

type (
	servingClient interface {
		InjectServingClient(servingclientset.Interface)
	}

	sharedClient interface {
		InjectSharedClient(sharedclientset.Interface)
	}

	cachingClient interface {
		InjectCachingClient(cachingclientset.Interface)
	}

	serviceLister interface {
		InjectServiceLister(servinglisters.ServiceLister)
	}

	configurationLister interface {
		InjectConfigurationLister(servinglisters.ConfigurationLister)
	}

	revisionLister interface {
		InjectRevisionLister(servinglisters.RevisionLister)
	}

	routeLister interface {
		InjectRouteLister(servinglisters.RouteLister)
	}

	podAutoscalerLister interface {
		InjectPodAutoscalerLister(kpalisters.PodAutoscalerLister)
	}

	virtualServiceLister interface {
		InjectVirtualServiceLister(istiolisters.VirtualServiceLister)
	}

	imageLister interface {
		InjectImageLister(cachinglisters.ImageLister)
	}
)

func ServingClientInto(c servingclientset.Interface, i interface{}) {
	if o, ok := i.(servingClient); ok {
		o.InjectServingClient(c)
	}
}

func SharedClientInto(c sharedclientset.Interface, i interface{}) {
	if o, ok := i.(sharedClient); ok {
		o.InjectSharedClient(c)
	}
}

func CachingClientInto(c cachingclientset.Interface, i interface{}) {
	if o, ok := i.(cachingClient); ok {
		o.InjectCachingClient(c)
	}
}

func ServiceListerInto(s servinglisters.ServiceLister, i interface{}) {
	if o, ok := i.(serviceLister); ok {
		o.InjectServiceLister(s)
	}
}

func ConfigurationListerInto(s servinglisters.ConfigurationLister, i interface{}) {
	if o, ok := i.(configurationLister); ok {
		o.InjectConfigurationLister(s)
	}
}

func RevisionListerInto(s servinglisters.RevisionLister, i interface{}) {
	if o, ok := i.(revisionLister); ok {
		o.InjectRevisionLister(s)
	}
}

func RouteListerInto(s servinglisters.RouteLister, i interface{}) {
	if o, ok := i.(routeLister); ok {
		o.InjectRouteLister(s)
	}
}

func PodAutoscalerListerInto(s kpalisters.PodAutoscalerLister, i interface{}) {
	if o, ok := i.(podAutoscalerLister); ok {
		o.InjectPodAutoscalerLister(s)
	}
}

func VirtualServiceListerInto(s istiolisters.VirtualServiceLister, i interface{}) {
	if o, ok := i.(virtualServiceLister); ok {
		o.InjectVirtualServiceLister(s)
	}
}

func ImageListerInto(s cachinglisters.ImageLister, i interface{}) {
	if o, ok := i.(imageLister); ok {
		o.InjectImageLister(s)
	}
}

func CachingListersInto(inf cachinginformers.SharedInformerFactory, i interface{}) {
	if o, ok := i.(imageLister); ok {
		o.InjectImageLister(inf.Caching().V1alpha1().Images().Lister())
	}
}

func SharedListersInto(inf sharedinformers.SharedInformerFactory, i interface{}) {
	if o, ok := i.(virtualServiceLister); ok {
		o.InjectVirtualServiceLister(inf.Networking().V1alpha3().VirtualServices().Lister())
	}
}

func ServingListersInto(inf servinginformers.SharedInformerFactory, i interface{}) {
	if o, ok := i.(serviceLister); ok {
		o.InjectServiceLister(inf.Serving().V1alpha1().Services().Lister())
	}

	if o, ok := i.(configurationLister); ok {
		o.InjectConfigurationLister(inf.Serving().V1alpha1().Configurations().Lister())
	}

	if o, ok := i.(revisionLister); ok {
		o.InjectRevisionLister(inf.Serving().V1alpha1().Revisions().Lister())
	}

	if o, ok := i.(routeLister); ok {
		o.InjectRouteLister(inf.Serving().V1alpha1().Routes().Lister())
	}

	if o, ok := i.(podAutoscalerLister); ok {
		o.InjectPodAutoscalerLister(inf.Autoscaling().V1alpha1().PodAutoscalers().Lister())
	}
}
