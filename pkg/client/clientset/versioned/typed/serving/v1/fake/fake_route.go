/*
Copyright 2022 The Knative Authors

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	gentype "k8s.io/client-go/gentype"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	servingv1 "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1"
)

// fakeRoutes implements RouteInterface
type fakeRoutes struct {
	*gentype.FakeClientWithList[*v1.Route, *v1.RouteList]
	Fake *FakeServingV1
}

func newFakeRoutes(fake *FakeServingV1, namespace string) servingv1.RouteInterface {
	return &fakeRoutes{
		gentype.NewFakeClientWithList[*v1.Route, *v1.RouteList](
			fake.Fake,
			namespace,
			v1.SchemeGroupVersion.WithResource("routes"),
			v1.SchemeGroupVersion.WithKind("Route"),
			func() *v1.Route { return &v1.Route{} },
			func() *v1.RouteList { return &v1.RouteList{} },
			func(dst, src *v1.RouteList) { dst.ListMeta = src.ListMeta },
			func(list *v1.RouteList) []*v1.Route { return gentype.ToPointerSlice(list.Items) },
			func(list *v1.RouteList, items []*v1.Route) { list.Items = gentype.FromPointerSlice(items) },
		),
		fake,
	}
}
