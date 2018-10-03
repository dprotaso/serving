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

package phase

import (
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/knative/serving/pkg/reconciler/v1alpha1/testing"
)

func TestServiceReconcile(t *testing.T) {
	scenarios := PhaseTests{
		{
			Name:     "first-reconcile",
			Resource: simpleRunLatest("default", "first-reconcile", "not-ready", nil),
			ExpectedStatus: v1alpha1.RouteStatus{
				DomainInternal: "first-reconcile.default.svc.cluster.local",
				Targetable:     &duckv1alpha1.Targetable{DomainInternal: "first-reconcile.default.svc.cluster.local"},
			},
			ExpectedCreates: Creates{
				resources.MakeK8sService(
					simpleRunLatest("default", "first-reconcile", "not-ready", nil),
				),
			},
		}, {
			Name:     "create-service-fails",
			Resource: simpleRunLatest("default", "first-reconcile", "not-ready", nil),
			Failures: Failures{
				InduceFailure("create", "services"),
			},
			ExpectError:    true,
			ExpectedStatus: NoStatusChange,
			ExpectedCreates: Creates{
				resources.MakeK8sService(
					simpleRunLatest("default", "first-reconcile", "not-ready", nil),
				),
			},
		}, {
			Name:     "steady-state",
			Resource: simpleRunLatest("default", "steady-state", "not-ready", nil),
			Objects: Objects{
				resources.MakeK8sService(
					simpleRunLatest("default", "steady-state", "not-ready", nil),
				),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				DomainInternal: "steady-state.default.svc.cluster.local",
				Targetable:     &duckv1alpha1.Targetable{DomainInternal: "steady-state.default.svc.cluster.local"},
			},
		}, {
			Name:     "service-spec-change",
			Resource: simpleRunLatest("default", "service-change", "not-ready", nil),
			Objects: Objects{
				mutateService(
					resources.MakeK8sService(
						simpleRunLatest("default", "service-change", "not-ready", nil),
					),
				),
			},
			ExpectedUpdates: Updates{
				resources.MakeK8sService(
					simpleRunLatest("default", "service-change", "not-ready", nil),
				),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				DomainInternal: "service-change.default.svc.cluster.local",
				Targetable:     &duckv1alpha1.Targetable{DomainInternal: "service-change.default.svc.cluster.local"},
			},
		}, {
			Name:     "service-update-failed",
			Resource: simpleRunLatest("default", "service-change", "not-ready", nil),
			Objects: Objects{
				mutateService(
					resources.MakeK8sService(
						simpleRunLatest("default", "service-change", "not-ready", nil),
					),
				),
			},
			Failures: Failures{
				InduceFailure("update", "services"),
			},
			ExpectError:    true,
			ExpectedStatus: NoStatusChange,
			ExpectedUpdates: Updates{
				resources.MakeK8sService(
					simpleRunLatest("default", "service-change", "not-ready", nil),
				),
			},
		}, {
			Name:     "allow cluster ip",
			Resource: simpleRunLatest("default", "cluster-ip", "config", nil),
			Objects: Objects{
				setClusterIP(
					resources.MakeK8sService(
						simpleRunLatest("default", "cluster-ip", "config", nil),
					),
					"127.0.0.1",
				),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				DomainInternal: "cluster-ip.default.svc.cluster.local",
				Targetable:     &duckv1alpha1.Targetable{DomainInternal: "cluster-ip.default.svc.cluster.local"},
			},
		},
	}

	scenarios.Run(t, PhaseSetup, K8sService{})
}

func mutateService(svc *corev1.Service) *corev1.Service {
	// Thor's Hammer
	svc.Spec = corev1.ServiceSpec{}
	return svc
}

func simpleRunLatest(namespace, name, config string, status *v1alpha1.RouteStatus) *v1alpha1.Route {
	return routeWithTraffic(namespace, name, status, v1alpha1.TrafficTarget{
		ConfigurationName: config,
		Percent:           100,
	})
}

func routeWithTraffic(namespace, name string, status *v1alpha1.RouteStatus, traffic ...v1alpha1.TrafficTarget) *v1alpha1.Route {
	route := &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.RouteSpec{
			Traffic: traffic,
		},
	}
	if status != nil {
		route.Status = *status
	}
	return route
}

func setClusterIP(svc *corev1.Service, ip string) *corev1.Service {
	svc.Spec.ClusterIP = ip
	return svc
}
