/*
Copyright 2018 The Knative Authors.

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
	"fmt"
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/traffic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	. "github.com/knative/serving/pkg/reconciler/v1alpha1/testing"
)

func TestVirtualServiceReconcile(t *testing.T) {
	scenarios := PhaseTests{{
		// When the configuration is not ready there should be
		Name:     "configuration not yet ready",
		Resource: simpleRunLatest("default", "first-reconcile", "not-ready", nil),
		Objects: Objects{
			simpleNotReadyConfig("default", "not-ready"),
			simpleNotReadyRevision("default",
				// Use the Revision name from the config.
				simpleNotReadyConfig("default", "not-ready").Status.LatestCreatedRevisionName,
			),
		},
		ExpectedPatches: Patches{
			patchAddLabel("default", "not-ready", "serving.knative.dev/route", "first-reconcile", "v1"),
		},
		ExpectedStatus: v1alpha1.RouteStatus{
			Conditions: duckv1alpha1.Conditions{{
				Type:    v1alpha1.RouteConditionAllTrafficAssigned,
				Status:  corev1.ConditionUnknown,
				Reason:  "RevisionMissing",
				Message: `Configuration "not-ready" is waiting for a Revision to become ready.`,
			}, {
				Type:    v1alpha1.RouteConditionReady,
				Status:  corev1.ConditionUnknown,
				Reason:  "RevisionMissing",
				Message: `Configuration "not-ready" is waiting for a Revision to become ready.`,
			}},
		},
	}, {
		Name:     "configuration permanently failed",
		Resource: simpleRunLatest("default", "first-reconcile", "permanently-failed", nil),
		Objects: []runtime.Object{
			simpleFailedConfig("default", "permanently-failed"),
			simpleFailedRevision("default",
				// Use the Revision name from the config.
				simpleFailedConfig("default", "permanently-failed").Status.LatestCreatedRevisionName,
			),
		},
		ExpectedPatches: Patches{
			patchAddLabel("default", "permanently-failed", "serving.knative.dev/route", "first-reconcile", "v1"),
		},
		ExpectedStatus: v1alpha1.RouteStatus{
			Conditions: duckv1alpha1.Conditions{{
				Type:    v1alpha1.RouteConditionAllTrafficAssigned,
				Status:  corev1.ConditionFalse,
				Reason:  "RevisionMissing",
				Message: `Configuration "permanently-failed" does not have any ready Revision.`,
			}, {
				Type:    v1alpha1.RouteConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "RevisionMissing",
				Message: `Configuration "permanently-failed" does not have any ready Revision.`,
			}},
		},
	}, {
		Name: "simple route becomes ready",
		Resource: setDomain(
			simpleRunLatest("default", "becomes-ready", "config", nil),
			"becomes-ready.default.example.com",
		),
		Objects: Objects{
			simpleReadyConfig("default", "config"),
			simpleReadyRevision("default",
				// Use the Revision name from the config.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
			),
		},
		ExpectedCreates: Creates{
			resources.MakeVirtualService(
				setDomain(
					simpleRunLatest("default", "becomes-ready", "config", nil),
					"becomes-ready.default.example.com",
				),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": {{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		},
		ExpectedPatches: Patches{
			patchAddLabel("default", "config", "serving.knative.dev/route", "becomes-ready", "v1"),
		},
		ExpectedStatus: v1alpha1.RouteStatus{
			// TODO(dprotaso) 'Domain' value comes from the route test fixture
			Domain: "becomes-ready.default.example.com",
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RouteConditionAllTrafficAssigned,
				Status: corev1.ConditionTrue,
			}, {
				Type:   v1alpha1.RouteConditionReady,
				Status: corev1.ConditionTrue,
			}},
			Traffic: []v1alpha1.TrafficTarget{{
				RevisionName: "config-00001",
				Percent:      100,
			}},
		},
	}, {
		Name: "failure labeling configuration",
		// Start from the test case where the route becomes ready and introduce a failure updating the configuration.
		Resource: simpleRunLatest("default", "label-config-failure", "config", nil),
		Failures: Failures{
			InduceFailure("patch", "configurations"),
		},
		Objects: Objects{
			simpleReadyConfig("default", "config"),
			simpleReadyRevision("default",
				// Use the Revision name from the config.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
			),
		},
		ExpectError:    true,
		ExpectedStatus: NoStatusChange,
		ExpectedPatches: Patches{
			patchAddLabel("default", "config", "serving.knative.dev/route", "label-config-failure", "v1"),
		},
	}, {
		Name: "failure creating virtual service",
		Resource: setDomain(
			simpleRunLatest("default", "vs-create-failure", "config", nil),
			"vs-create-failure.default.example.com",
		),
		Objects: Objects{
			simpleReadyConfig("default", "config"),
			simpleReadyRevision("default",
				// Use the Revision name from the config.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
			),
		},
		// We induce a failure creating the VirtualService.
		Failures: Failures{
			InduceFailure("create", "virtualservices"),
		},
		ExpectError:    true,
		ExpectedStatus: NoStatusChange,
		ExpectedCreates: Creates{
			// This is the Create we see for the virtual service, but we induce a failure.
			resources.MakeVirtualService(
				setDomain(
					simpleRunLatest("default", "vs-create-failure", "config", nil),
					"vs-create-failure.default.example.com",
				),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": {{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		},
		ExpectedPatches: Patches{
			patchAddLabel("default", "config", "serving.knative.dev/route", "vs-create-failure", "v1"),
		},
	}, {
		Name: "steady state",
		Resource: simpleRunLatest("default", "steady-state", "config", &v1alpha1.RouteStatus{
			Domain:         "steady-state.default.example.com",
			DomainInternal: "steady-state.default.svc.cluster.local",
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RouteConditionAllTrafficAssigned,
				Status: corev1.ConditionTrue,
			}, {
				Type:   v1alpha1.RouteConditionReady,
				Status: corev1.ConditionTrue,
			}},
			Traffic: []v1alpha1.TrafficTarget{{
				RevisionName: "config-00001",
				Percent:      100,
			}},
		}),
		Objects: Objects{
			addConfigLabel(
				simpleReadyConfig("default", "config"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "steady-state",
			),
			simpleReadyRevision("default",
				// Use the Revision name from the config.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
			),
			resources.MakeVirtualService(
				setDomain(
					simpleRunLatest("default", "steady-state", "config", nil),
					"steady-state.default.example.com",
				),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": {{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		},
		ExpectedStatus: NoStatusChange,
	}, {
		Name: "different domain",
		Resource: simpleRunLatest("default", "different-domain", "config", &v1alpha1.RouteStatus{
			// Previously the the service was reconciled with 'example.com'
			Domain:         "different-domain.default.another-example.com",
			DomainInternal: "different-domain.default.svc.cluster.local",
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RouteConditionAllTrafficAssigned,
				Status: corev1.ConditionTrue,
			}, {
				Type:   v1alpha1.RouteConditionReady,
				Status: corev1.ConditionTrue,
			}},
			Traffic: []v1alpha1.TrafficTarget{{
				RevisionName: "config-00001",
				Percent:      100,
			}},
		}),
		Objects: Objects{
			addConfigLabel(
				simpleReadyConfig("default", "config"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "different-domain",
			),
			simpleReadyRevision("default",
				// Use the Revision name from the config.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
			),
			resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "different-domain", "config", nil), "different-domain.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": {{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		},
		ExpectedUpdates: Updates{
			resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "different-domain", "config", nil), "different-domain.default.another-example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": {{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		},
		ExpectedStatus: v1alpha1.RouteStatus{
			// TODO(dprotaso) 'Domain' value comes from the route test fixture
			Domain: "different-domain.default.another-example.com",
			// TODO(dprotaso) 'DomainInternal' value comes from the route test fixture
			DomainInternal: "different-domain.default.svc.cluster.local",
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RouteConditionAllTrafficAssigned,
				Status: corev1.ConditionTrue,
			}, {
				Type:   v1alpha1.RouteConditionReady,
				Status: corev1.ConditionTrue,
			}},
			Traffic: []v1alpha1.TrafficTarget{{
				RevisionName: "config-00001",
				Percent:      100,
			}},
		},
	},
		{
			// A new LatestCreatedRevisionName on the Configuration alone should result in no changes to the Route.
			Name: "new latest created revision",
			Resource: simpleRunLatest("default", "new-latest-created", "config", &v1alpha1.RouteStatus{
				Domain:         "new-latest-created.default.example.com",
				DomainInternal: "new-latest-created.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				setLatestCreatedRevision(
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "new-latest-created",
					),
					"config-00002",
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
				),
				// This is the name of the new revision we're referencing above.
				simpleNotReadyRevision("default", "config-00002"),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "new-latest-created", "config", nil), "new-latest-created.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: NoStatusChange,
		}, {
			Name: "new latest ready revision",
			Resource: simpleRunLatest("default", "new-latest-ready", "config", &v1alpha1.RouteStatus{
				Domain:         "new-latest-ready.default.example.com",
				DomainInternal: "new-latest-ready.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				setLatestReadyRevision(setLatestCreatedRevision(
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "new-latest-ready",
					),
					"config-00002",
				)),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
				),
				// This is the name of the new revision we're referencing above.
				simpleReadyRevision("default", "config-00002"),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "new-latest-ready", "config", nil), "new-latest-ready.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			// A new LatestReadyRevisionName on the Configuration should result in the new Revision being rolled out.
			ExpectedUpdates: Updates{
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "new-latest-ready", "config", nil), "new-latest-ready.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// This is the new config we're making become ready.
									RevisionName: "config-00002",
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				// TODO(dprotaso) 'Domain' value comes from the route test fixture
				Domain: "new-latest-ready.default.example.com",
				// TODO(dprotaso) 'DomainInternal' value comes from the route test fixture
				DomainInternal: "new-latest-ready.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00002",
					Percent:      100,
				}},
			},
		}, {
			Name: "failure updating virtual service",
			// Starting from the new latest ready, induce a failure updating the virtual service.
			ExpectError: true,
			Failures: Failures{
				InduceFailure("update", "virtualservices"),
			},
			Resource: simpleRunLatest("default", "update-vs-failure", "config", &v1alpha1.RouteStatus{
				Domain:         "update-vs-failure.default.example.com",
				DomainInternal: "update-vs-failure.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				setLatestReadyRevision(setLatestCreatedRevision(
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "update-vs-failure",
					),
					"config-00002",
				)),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
				),
				// This is the name of the new revision we're referencing above.
				simpleReadyRevision("default", "config-00002"),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "update-vs-failure", "config", nil), "update-vs-failure.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
				resources.MakeK8sService(simpleRunLatest("default", "update-vs-failure", "config", nil)),
			},
			ExpectedUpdates: Updates{
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "update-vs-failure", "config", nil), "update-vs-failure.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// This is the new config we're making become ready.
									RevisionName: "config-00002",
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: NoStatusChange,
		}, {
			Name: "reconcile virtual service mutation",
			Resource: simpleRunLatest("default", "virt-svc-mutation", "config", &v1alpha1.RouteStatus{
				Domain:         "virt-svc-mutation.default.example.com",
				DomainInternal: "virt-svc-mutation.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				addConfigLabel(
					simpleReadyConfig("default", "config"),
					// The Route controller attaches our label to this Configuration.
					"serving.knative.dev/route", "virt-svc-mutation",
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
				),
				mutateVirtualService(resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "virt-svc-mutation", "config", nil), "virt-svc-mutation.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				)),
			},
			ExpectedUpdates: Updates{
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "virt-svc-mutation", "config", nil), "virt-svc-mutation.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: NoStatusChange,
		}, {
			Name:        "config labelled by another route",
			ExpectError: true,
			Resource: simpleRunLatest("default", "licked-cookie", "config", &v1alpha1.RouteStatus{
				Domain:         "licked-cookie.default.example.com",
				DomainInternal: "licked-cookie.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "config-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				addConfigLabel(
					simpleReadyConfig("default", "config"),
					// This configuration is being referenced by another Route.
					"serving.knative.dev/route", "this-cookie-has-been-licked",
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
				),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "licked-cookie", "config", nil), "licked-cookie.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: NoStatusChange,
		}, {
			Name: "switch to a different config",
			// The status reflects "oldconfig", but the spec "newconfig".
			Resource: simpleRunLatest("default", "change-configs", "newconfig", &v1alpha1.RouteStatus{
				Domain:         "change-configs.default.example.com",
				DomainInternal: "change-configs.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "oldconfig-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				// Both configs exist, but only "oldconfig" is labelled.
				addConfigLabel(
					simpleReadyConfig("default", "oldconfig"),
					// The Route controller attaches our label to this Configuration.
					"serving.knative.dev/route", "change-configs",
				),
				simpleReadyConfig("default", "newconfig"),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "oldconfig").Status.LatestReadyRevisionName,
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "newconfig").Status.LatestReadyRevisionName,
				),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "change-configs", "oldconfig", nil), "change-configs.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "oldconfig").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
				resources.MakeK8sService(simpleRunLatest("default", "change-configs", "oldconfig", nil)),
			},
			ExpectedPatches: Patches{
				patchRemoveLabel("default", "oldconfig", "serving.knative.dev/route", "v1"),
				patchAddLabel("default", "newconfig", "serving.knative.dev/route", "change-configs", "v1"),
			},
			ExpectedUpdates: Updates{
				// Updated to point to "newconfig" things.
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "change-configs", "newconfig", nil), "change-configs.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "newconfig").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			// Status updated to "newconfig"
			ExpectedStatus: v1alpha1.RouteStatus{
				// TODO(dprotaso) 'Domain' value comes from the route test fixture
				Domain: "change-configs.default.example.com",
				// TODO(dprotaso) 'DomainInternal' value comes from the route test fixture
				DomainInternal: "change-configs.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "newconfig-00001",
					Percent:      100,
				}},
			},
		}, {
			Name:     "configuration missing",
			Resource: simpleRunLatest("default", "config-missing", "not-found", nil),
			ExpectedStatus: v1alpha1.RouteStatus{
				Conditions: duckv1alpha1.Conditions{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Configuration "not-found" referenced in traffic not found.`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Configuration "not-found" referenced in traffic not found.`,
				}},
			},
		}, {
			Name:     "revision missing (direct)",
			Resource: simplePinned("default", "missing-revision-direct", "not-found", nil),
			Objects: Objects{
				simpleReadyConfig("default", "config"),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				Conditions: duckv1alpha1.Conditions{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Revision "not-found" referenced in traffic not found.`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Revision "not-found" referenced in traffic not found.`,
				}},
			},
		}, {
			Name:     "revision missing (indirect)",
			Resource: simpleRunLatest("default", "missing-revision-indirect", "config", nil),
			Objects: Objects{
				simpleReadyConfig("default", "config"),
			},
			ExpectedPatches: Patches{
				patchAddLabel("default", "config", "serving.knative.dev/route", "missing-revision-indirect", "v1"),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				Conditions: duckv1alpha1.Conditions{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Revision "config-00001" referenced in traffic not found.`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Revision "config-00001" referenced in traffic not found.`,
				}},
			},
		}, {
			Name: "pinned route becomes ready",
			Resource: setDomain(
				simplePinned(
					"default",
					"pinned-becomes-ready",
					// Use the Revision name from the config
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					nil,
				),
				"pinned-becomes-ready.default.example.com",
			),
			Objects: Objects{
				simpleReadyConfig("default", "config"),
				addOwnerRef(
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
					or("Configuration", "config"),
				),
			},
			ExpectedCreates: Creates{
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "pinned-becomes-ready", "config", nil), "pinned-becomes-ready.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedPatches: Patches{
				// TODO(#1495): The parent configuration isn't labeled because it's established through
				// labels instead of owner references.
				//patchAddLabel("default", "config", "serving.knative.dev/route", "pinned-becomes-ready"),
			},
			// Use the config's revision name.
			ExpectedStatus: v1alpha1.RouteStatus{
				// TODO(dprotaso) 'Domain' value comes from the route test fixture
				Domain: "pinned-becomes-ready.default.example.com",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					// TODO(#1495): This is established thru labels instead of OwnerReferences.
					// ConfigurationName: "config",
					RevisionName: "config-00001",
					Percent:      100,
				}},
			},
		}, {
			Name: "traffic split becomes ready",
			Resource: setDomain(
				routeWithTraffic("default", "named-traffic-split", nil,
					v1alpha1.TrafficTarget{
						ConfigurationName: "blue",
						Percent:           50,
					}, v1alpha1.TrafficTarget{
						ConfigurationName: "green",
						Percent:           50,
					}),
				"named-traffic-split.default.example.com",
			),
			Objects: Objects{
				simpleReadyConfig("default", "blue"),
				simpleReadyConfig("default", "green"),
				addOwnerRef(
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
					),
					or("Configuration", "blue"),
				),
				addOwnerRef(
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
					),
					or("Configuration", "green"),
				),
			},
			ExpectedCreates: Creates{
				resources.MakeVirtualService(
					setDomain(routeWithTraffic("default", "named-traffic-split", nil,
						v1alpha1.TrafficTarget{
							ConfigurationName: "blue",
							Percent:           50,
						}, v1alpha1.TrafficTarget{
							ConfigurationName: "green",
							Percent:           50,
						}), "named-traffic-split.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
									Percent:      50,
								},
								Active: true,
							}, {
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
									Percent:      50,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedPatches: Patches{
				patchAddLabel("default", "blue", "serving.knative.dev/route", "named-traffic-split", "v1"),
				patchAddLabel("default", "green", "serving.knative.dev/route", "named-traffic-split", "v1"),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				// TODO(dprotaso) 'Domain' value comes from the route test fixture
				Domain: "named-traffic-split.default.example.com",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "blue-00001",
					Percent:      50,
				}, {
					RevisionName: "green-00001",
					Percent:      50,
				}},
			},
		}, {
			Name: "change route configuration",
			// Start from a steady state referencing "blue", and modify the route spec to point to "green" instead.
			Resource: simpleRunLatest("default", "switch-configs", "green", &v1alpha1.RouteStatus{
				Domain: "switch-configs.default.example.com",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "blue-00001",
					Percent:      100,
				}},
			}),
			Objects: Objects{
				addConfigLabel(
					simpleReadyConfig("default", "blue"),
					// The Route controller attaches our label to this Configuration.
					"serving.knative.dev/route", "switch-configs",
				),
				simpleReadyConfig("default", "green"),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
				),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "switch-configs", "blue", nil), "switch-configs.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
				resources.MakeK8sService(simpleRunLatest("default", "switch-configs", "blue", nil)),
			},
			ExpectedPatches: Patches{
				patchRemoveLabel("default", "blue", "serving.knative.dev/route", "v1"),
				patchAddLabel("default", "green", "serving.knative.dev/route", "switch-configs", "v1"),
			},
			ExpectedUpdates: Updates{
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "switch-configs", "green", nil), "switch-configs.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: v1alpha1.RouteStatus{
				// TODO(dprotaso) 'Domain' value comes from the route test fixture
				Domain: "switch-configs.default.example.com",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "green-00001",
					Percent:      100,
				}},
			},
		}, {
			Name: "failure unlabeling old configuration",
			// Start from our test that switches configs and induce a failure when we go to unlabel
			// the "blue" configuration.
			ExpectError: true,
			Failures: Failures{
				InduceFailure("patch", "configurations"),
			},
			Resource: simpleRunLatest("default", "rmlabel-config-failure", "green", &v1alpha1.RouteStatus{
				Domain:         "rmlabel-config-failure.default.example.com",
				DomainInternal: "rmlabel-config-failure.default.svc.cluster.local",
				Conditions: duckv1alpha1.Conditions{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					RevisionName: "blue-00001",
					Percent:      100,
				}},
			}),
			Objects: []runtime.Object{
				addConfigLabel(
					simpleReadyConfig("default", "blue"),
					// The Route controller attaches our label to this Configuration.
					"serving.knative.dev/route", "rmlabel-config-failure",
				),
				simpleReadyConfig("default", "green"),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
				),
				simpleReadyRevision("default",
					// Use the Revision name from the config.
					simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
				),
				resources.MakeVirtualService(
					setDomain(simpleRunLatest("default", "rmlabel-config-failure", "blue", nil), "rmlabel-config-failure.default.example.com"),
					&traffic.TrafficConfig{
						Targets: map[string][]traffic.RevisionTarget{
							"": {{
								TrafficTarget: v1alpha1.TrafficTarget{
									// Use the Revision name from the config.
									RevisionName: simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
									Percent:      100,
								},
								Active: true,
							}},
						},
					},
				),
			},
			ExpectedStatus: NoStatusChange,
			ExpectedPatches: Patches{
				patchRemoveLabel("default", "blue", "serving.knative.dev/route", "v1"),
			},
		}}

	scenarios.Run(t, VSPhaseSetup, VirtualService{})
}

// TODO(dprotaso)Review this alternate phase scenario invocation
func TestVirtualServiceReconcile_FailureLabellingConfiguration(t *testing.T) {
	route := simpleRunLatest("default", "addlabel-config-failure", "green", &v1alpha1.RouteStatus{
		Domain:         "addlabel-config-failure.default.example.com",
		DomainInternal: "addlabel-config-failure.default.svc.cluster.local",
		Conditions: duckv1alpha1.Conditions{{
			Type:   v1alpha1.RouteConditionAllTrafficAssigned,
			Status: corev1.ConditionTrue,
		}, {
			Type:   v1alpha1.RouteConditionReady,
			Status: corev1.ConditionTrue,
		}},
		Traffic: []v1alpha1.TrafficTarget{{
			RevisionName: "blue-00001",
			Percent:      100,
		}},
	})

	oldRoute := setDomain(
		simpleRunLatest("default", "addlabel-config-failure", "blue", nil),
		"addlabel-config-failure.default.example.com",
	)

	blueConfig := simpleReadyConfig("default", "blue")
	greenConfig := simpleReadyConfig("default", "green")
	blueRevision := simpleReadyRevision("default", blueConfig.Status.LatestCreatedRevisionName)
	greenRevision := simpleReadyRevision("default", greenConfig.Status.LatestCreatedRevisionName)
	virtualService := resources.MakeVirtualService(
		oldRoute,
		&traffic.TrafficConfig{
			Targets: map[string][]traffic.RevisionTarget{
				"": {{
					TrafficTarget: v1alpha1.TrafficTarget{
						// Use the Revision name from the config.
						RevisionName: simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
						Percent:      100,
					},
					Active: true,
				}},
			},
		},
	)

	scenario := PhaseTest{
		Name: "failure labeling configuration",
		// Start from our test that switches configs, unlabel "blue" (avoids induced failure),
		// and induce a failure when we go to label the "green" configuration.
		ExpectError: true,
		Failures: Failures{
			InduceFailure("patch", "configurations"),
		},
		Resource: route,
		Objects: Objects{
			blueConfig,
			blueRevision,
			greenConfig,
			greenRevision,
			virtualService,
		},
		ExpectedPatches: Patches{
			patchAddLabel("default", "green", "serving.knative.dev/route", "addlabel-config-failure", "v1"),
		},
		ExpectedStatus: NoStatusChange,
	}

	scenario.Run(t, VSPhaseSetup, VirtualService{})
}

func VSPhaseSetup(obj interface{}, objs []runtime.Object) []FakeClient {
	vs := obj.(*VirtualService)
	vs.Tracker = &NullTracker{}
	return PhaseSetup(obj, objs)
}

func patchRemoveLabel(namespace, name, key, version string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	patch := fmt.Sprintf(`{"metadata":{"labels":{"%s":null},"resourceVersion":"%s"}}`, key, version)

	action.Patch = []byte(patch)
	return action
}

func patchAddLabel(namespace, name, key, value, version string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace

	patch := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"},"resourceVersion":"%s"}}`, key, value, version)

	action.Patch = []byte(patch)
	return action
}

func simpleNotReadyConfig(namespace, name string) *v1alpha1.Configuration {
	cfg := &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
			ResourceVersion: "v1",
		},
		Spec: v1alpha1.ConfigurationSpec{
			Generation: 1,
			RevisionTemplate: v1alpha1.RevisionTemplateSpec{
				Spec: v1alpha1.RevisionSpec{
					Container: corev1.Container{
						Image: "busybox",
					},
				},
			},
		},
	}
	cfg.Status.InitializeConditions()
	cfg.Status.SetLatestCreatedRevisionName(name + "-00001")
	return cfg
}

func simpleNotReadyRevision(namespace, name string) *v1alpha1.Revision {
	return &v1alpha1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1alpha1.RevisionStatus{
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RevisionConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	}
}

func simpleFailedConfig(namespace, name string) *v1alpha1.Configuration {
	cfg := &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            name,
			ResourceVersion: "v1",
		},
		Spec: v1alpha1.ConfigurationSpec{
			Generation: 1,
			RevisionTemplate: v1alpha1.RevisionTemplateSpec{
				Spec: v1alpha1.RevisionSpec{
					Container: corev1.Container{
						Image: "busybox",
					},
				},
			},
		},
	}
	cfg.Status.InitializeConditions()
	cfg.Status.MarkLatestCreatedFailed(name+"-00001", "should have used ko")
	return cfg
}

func simpleFailedRevision(namespace, name string) *v1alpha1.Revision {
	return &v1alpha1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1alpha1.RevisionStatus{
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RevisionConditionReady,
				Status: corev1.ConditionFalse,
			}},
		},
	}
}

func simpleReadyConfig(namespace, name string) *v1alpha1.Configuration {
	return setLatestReadyRevision(simpleNotReadyConfig(namespace, name))
}

func setLatestCreatedRevision(cfg *v1alpha1.Configuration, name string) *v1alpha1.Configuration {
	cfg.Status.SetLatestCreatedRevisionName(name)
	return cfg
}

func setLatestReadyRevision(cfg *v1alpha1.Configuration) *v1alpha1.Configuration {
	cfg.Status.SetLatestReadyRevisionName(cfg.Status.LatestCreatedRevisionName)
	return cfg
}

func simpleReadyRevision(namespace, name string) *v1alpha1.Revision {
	return &v1alpha1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1alpha1.RevisionStatus{
			Conditions: duckv1alpha1.Conditions{{
				Type:   v1alpha1.RevisionConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}

func setDomain(route *v1alpha1.Route, domain string) *v1alpha1.Route {
	route.Status.Domain = domain
	return route
}

func addConfigLabel(config *v1alpha1.Configuration, key, value string) *v1alpha1.Configuration {
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	config.Labels[key] = value
	return config
}

func mutateVirtualService(vs *istiov1alpha3.VirtualService) *istiov1alpha3.VirtualService {
	// Thor's Hammer
	vs.Spec = istiov1alpha3.VirtualServiceSpec{}
	return vs
}

func simplePinned(namespace, name, revision string, status *v1alpha1.RouteStatus) *v1alpha1.Route {
	return routeWithTraffic(namespace, name, status, v1alpha1.TrafficTarget{
		RevisionName: revision,
		Percent:      100,
	})
}

func addOwnerRef(rev *v1alpha1.Revision, o []metav1.OwnerReference) *v1alpha1.Revision {
	rev.OwnerReferences = o
	return rev
}

// or builds OwnerReferences for a child of a Service
func or(kind, name string) []metav1.OwnerReference {
	boolTrue := true
	return []metav1.OwnerReference{{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               kind,
		Name:               name,
		Controller:         &boolTrue,
		BlockOwnerDeletion: &boolTrue,
	}}
}
