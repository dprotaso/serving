/*
Copyright 2019 The Knative Authors

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

package net

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/networking/pkg/apis/networking"
)

func TestEndpointsToDests(t *testing.T) {
	for _, tc := range []struct {
		name           string
		endpointSlice  discoveryv1.EndpointSlice
		protocol       networking.ProtocolType
		expectReady    sets.Set[string]
		expectNotReady sets.Set[string]
	}{{
		name: "no endpoints",
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{},
		},
		expectReady: sets.New[string](),
	}, {
		name: "single endpoint single address",
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{{
				Name: func() *string { s := networking.ServicePortNameHTTP1; return &s }(),
				Port: func() *int32 { i := int32(1234); return &i }(),
			}},
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{"128.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: func() *bool { r := true; return &r }(),
				},
			}},
		},
		expectReady: sets.New("128.0.0.1:1234"),
	}, {
		name: "single endpoint multiple addresses",
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{{
				Name: func() *string { s := networking.ServicePortNameHTTP1; return &s }(),
				Port: func() *int32 { i := int32(1234); return &i }(),
			}},
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{"128.0.0.1", "128.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: func() *bool { r := true; return &r }(),
				},
			}},
		},
		expectReady: sets.New("128.0.0.1:1234", "128.0.0.2:1234"),
	}, {
		name: "single endpoint multiple addresses, including no ready addresses",
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{{
				Name: func() *string { s := networking.ServicePortNameHTTP1; return &s }(),
				Port: func() *int32 { i := int32(1234); return &i }(),
			}},
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{"128.0.0.1", "128.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: func() *bool { r := true; return &r }(),
				},
			}, {
				Addresses: []string{"128.0.0.3"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: func() *bool { r := false; return &r }(),
				},
			}},
		},
		expectReady:    sets.New("128.0.0.1:1234", "128.0.0.2:1234"),
		expectNotReady: sets.New("128.0.0.3:1234"),
	}, {
		name: "multiple endpoint filter port",
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{
				{
					Name: func() *string { s := networking.ServicePortNameHTTP1; return &s }(),
					Port: func() *int32 { i := int32(1234); return &i }(),
				},
				{
					Name: func() *string { s := "other-protocol"; return &s }(),
					Port: func() *int32 { i := int32(1234); return &i }(),
				},
			},
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{"128.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: func() *bool { r := true; return &r }(),
				},
			}},
		},
		expectReady: sets.New("128.0.0.1:1234"),
	}, {
		name:     "multiple endpoint, different protocol",
		protocol: networking.ProtocolH2C,
		endpointSlice: discoveryv1.EndpointSlice{
			Ports: []discoveryv1.EndpointPort{
				{
					Name: func() *string { s := networking.ServicePortNameHTTP1; return &s }(),
					Port: func() *int32 { i := int32(1234); return &i }(),
				},
				{
					Name: func() *string { s := networking.ServicePortNameH2C; return &s }(),
					Port: func() *int32 { i := int32(5678); return &i }(),
				},
			},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"128.0.0.1", "128.0.0.2"},
					Conditions: discoveryv1.EndpointConditions{
						Ready: func() *bool { r := true; return &r }(),
					},
				},
				{
					Addresses: []string{"128.0.0.3", "128.0.0.4"},
					Conditions: discoveryv1.EndpointConditions{
						Ready: func() *bool { r := true; return &r }(),
					},
				},
			},
		},
		expectReady: sets.New("128.0.0.1:5678", "128.0.0.2:5678", "128.0.0.3:5678", "128.0.0.4:5678"),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.protocol == "" {
				tc.protocol = networking.ProtocolHTTP1
			}
			ready, notReady := endpointsToDests(&tc.endpointSlice, networking.ServicePortName(tc.protocol))

			if got, want := ready, tc.expectReady; !got.Equal(want) {
				t.Error("Got unexpected ready dests (-want, +got):", cmp.Diff(want, got))
			}
			if got, want := notReady, tc.expectNotReady; !got.Equal(want) {
				t.Error("Got unexpected notReady dests (-want, +got):", cmp.Diff(want, got))
			}
		})
	}
}

func TestGetServicePort(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol networking.ProtocolType
		ports    []corev1.ServicePort
		expect   int
		expectOK bool
	}{{
		name:     "Single port",
		protocol: networking.ProtocolHTTP1,
		ports: []corev1.ServicePort{{
			Name: "http",
			Port: 100,
		}},
		expect:   100,
		expectOK: true,
	}, {
		name:     "Missing port",
		protocol: networking.ProtocolHTTP1,
		ports: []corev1.ServicePort{{
			Name: "invalid",
			Port: 100,
		}},
		expect:   0,
		expectOK: false,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: tc.ports,
				},
			}

			port, ok := getServicePort(tc.protocol, &svc)
			if ok != tc.expectOK {
				t.Errorf("Wanted ok %v, got %v", tc.expectOK, ok)
			}
			if port != tc.expect {
				t.Errorf("Wanted port %d, got port %d", tc.expect, port)
			}
		})
	}
}

func BenchmarkHealthyAddresses(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprint("addresses-", n), func(b *testing.B) {
			ep := endpointSlice(10, n)
			for range b.N {
				healthyAddresses(ep, networking.ServicePortNameHTTP1)
			}
		})
	}
}

func BenchmarkEndpointsToDests(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprint("addresses-", n), func(b *testing.B) {
			ep := endpointSlice(10, n)
			for range b.N {
				endpointsToDests(ep, networking.ServicePortNameHTTP1)
			}
		})
	}
}

func endpointSlice(activators, apps int) *discoveryv1.EndpointSlice {
	port := int32(1234)
	portName := networking.ServicePortNameHTTP1

	endpoints := make([]discoveryv1.Endpoint, 0, activators+apps*2)

	// Add ready activator endpoints
	for i := range activators {
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Addresses: []string{fmt.Sprintf("activator-%d", i)},
			Conditions: discoveryv1.EndpointConditions{
				Ready: func() *bool { r := true; return &r }(),
			},
		})
	}

	// Add ready app endpoints
	for i := range apps {
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Addresses: []string{fmt.Sprintf("app-%d", i)},
			Conditions: discoveryv1.EndpointConditions{
				Ready: func() *bool { r := true; return &r }(),
			},
		})
	}

	// Add not-ready app endpoints
	for i := range apps {
		endpoints = append(endpoints, discoveryv1.Endpoint{
			Addresses: []string{fmt.Sprintf("app-non-ready-%d", i)},
			Conditions: discoveryv1.EndpointConditions{
				Ready: func() *bool { r := false; return &r }(),
			},
		})
	}

	return &discoveryv1.EndpointSlice{
		Ports: []discoveryv1.EndpointPort{{
			Name: &portName,
			Port: &port,
		}},
		Endpoints: endpoints,
	}
}
