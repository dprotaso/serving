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

package v1

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	netv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	apistest "knative.dev/pkg/apis/testing"
	"knative.dev/serving/pkg/apis/serving"
)

func TestRouteDuckTypes(t *testing.T) {
	tests := []struct {
		name string
		t    duck.Implementable
	}{{
		name: "conditions",
		t:    &duckv1.Conditions{},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := duck.VerifyType(&Route{}, test.t)
			if err != nil {
				t.Errorf("VerifyType(Route, %T) = %v", test.t, err)
			}
		})
	}
}

func TestRouteGetConditionSet(t *testing.T) {
	r := &Route{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestRouteGetGroupVersionKind(t *testing.T) {
	r := &Route{}
	want := schema.GroupVersionKind{
		Group:   "serving.knative.dev",
		Version: "v1",
		Kind:    "Route",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestRouteIsReady(t *testing.T) {
	cases := []struct {
		name    string
		status  RouteStatus
		isReady bool
	}{{
		name:    "empty status should not be ready",
		status:  RouteStatus{},
		isReady: false,
	}, {
		name: "Different condition type should not be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		isReady: false,
	}, {
		name: "False condition status should not be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		isReady: false,
	}, {
		name: "Unknown condition status should not be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
		isReady: false,
	}, {
		name: "Missing condition status should not be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type: RouteConditionReady,
				}},
			},
		},
		isReady: false,
	}, {
		name: "True condition status should be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		isReady: true,
	}, {
		name: "Multiple conditions with ready status should be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		isReady: true,
	}, {
		name: "Multiple conditions with ready status false should not be ready",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   RouteConditionReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		isReady: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Route{Status: tc.status}
			if e, a := tc.isReady, r.IsReady(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}

			r.Generation = 1
			r.Status.ObservedGeneration = 2
			if r.IsReady() {
				t.Error("Expected IsReady() to be false when Generation != ObservedGeneration")
			}
		})
	}
}

func TestRouteIsFailed(t *testing.T) {
	cases := []struct {
		name     string
		status   RouteStatus
		isFailed bool
	}{{
		name:     "empty status should not be failed",
		status:   RouteStatus{},
		isFailed: false,
	}, {
		name: "False condition status should be failed",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		isFailed: true,
	}, {
		name: "Unknown condition status should not be failed",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
		isFailed: false,
	}, {
		name: "Missing condition status should not be failed",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type: RouteConditionReady,
				}},
			},
		},
		isFailed: false,
	}, {
		name: "True condition status should not be failed",
		status: RouteStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		isFailed: false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Route{Status: tc.status}
			if e, a := tc.isFailed, r.IsFailed(); e != a {
				t.Errorf("%q expected: %v got: %v", tc.name, e, a)
			}
		})
	}
}

func TestTypicalRouteFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkTrafficAssigned()
	r.MarkTLSNotEnabled(ExternalDomainTLSNotEnabledMessage)
	apistest.CheckConditionSucceeded(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	})
	apistest.CheckConditionSucceeded(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionSucceeded(r, RouteConditionIngressReady, t)
	apistest.CheckConditionSucceeded(r, RouteConditionReady, t)
}

func TestTrafficNotAssignedFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkMissingTrafficTarget("Revision", "does-not-exist")
	apistest.CheckConditionFailed(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionFailed(r, RouteConditionReady, t)
}

func TestTargetConfigurationNotYetReadyFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkConfigurationNotReady("i-have-no-ready-revision")
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)
}

func TestUnknownErrorWhenConfiguringTraffic(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkUnknownTrafficError("unknown-error")
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)
}

func TestTargetConfigurationFailedToBeReadyFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkConfigurationFailed("permanently-failed")
	apistest.CheckConditionFailed(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionFailed(r, RouteConditionReady, t)
}

func TestTargetRevisionNotYetReadyFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkRevisionNotReady("not-yet-ready")
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)
}

func TestTargetRevisionFailedToBeReadyFlow(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkRevisionFailed("cannot-find-image")
	apistest.CheckConditionFailed(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionFailed(r, RouteConditionReady, t)
}

func TestIngressFailureRecovery(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	})
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	// Empty IngressStatus marks ingress "NotConfigured"
	r.PropagateIngressStatus(netv1alpha1.IngressStatus{})
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkTrafficAssigned()
	r.MarkTLSNotEnabled(ExternalDomainTLSNotEnabledMessage)
	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	})
	apistest.CheckConditionSucceeded(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionSucceeded(r, RouteConditionIngressReady, t)
	apistest.CheckConditionSucceeded(r, RouteConditionReady, t)

	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionFalse,
			}},
		},
	})
	apistest.CheckConditionSucceeded(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionFailed(r, RouteConditionIngressReady, t)
	apistest.CheckConditionFailed(r, RouteConditionReady, t)

	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	})
	apistest.CheckConditionSucceeded(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionSucceeded(r, RouteConditionIngressReady, t)
	apistest.CheckConditionSucceeded(r, RouteConditionReady, t)
}

func TestRouteNotOwnedStuff(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.PropagateIngressStatus(netv1alpha1.IngressStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{{
				Type:   netv1alpha1.IngressConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
	})

	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
	apistest.CheckConditionOngoing(r, RouteConditionReady, t)

	r.MarkServiceNotOwned("evan")
	apistest.CheckConditionOngoing(r, RouteConditionAllTrafficAssigned, t)
	apistest.CheckConditionFailed(r, RouteConditionIngressReady, t)
	apistest.CheckConditionFailed(r, RouteConditionReady, t)
}

func TestCertificateProvision(t *testing.T) {
	message := "CommonName Too Long"

	cases := []struct {
		name   string
		cert   *netv1alpha1.Certificate
		status corev1.ConditionStatus

		wantMessage string
	}{{
		name:        "Ready with empty message",
		cert:        &netv1alpha1.Certificate{},
		status:      corev1.ConditionTrue,
		wantMessage: "",
	}, {
		name:        "NotReady with empty message",
		cert:        &netv1alpha1.Certificate{},
		status:      corev1.ConditionUnknown,
		wantMessage: "",
	}, {
		name: "NotReady with bubbled up message",
		cert: &netv1alpha1.Certificate{
			Status: netv1alpha1.CertificateStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionUnknown,
						Reason: message,
					}},
				},
			},
		},
		status:      corev1.ConditionUnknown,
		wantMessage: message,
	}, {
		name:        "Failed with empty message",
		cert:        &netv1alpha1.Certificate{},
		status:      corev1.ConditionFalse,
		wantMessage: "",
	}, {
		name: "Failed with bubbled up message",
		cert: &netv1alpha1.Certificate{
			Status: netv1alpha1.CertificateStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{{
						Type:   apis.ConditionReady,
						Status: corev1.ConditionFalse,
						Reason: message,
					}},
				},
			},
		},
		status:      corev1.ConditionFalse,
		wantMessage: message,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &RouteStatus{}
			r.InitializeConditions()

			switch tc.status {
			case corev1.ConditionTrue:
				r.MarkCertificateReady(tc.cert.Name)
			case corev1.ConditionFalse:
				r.MarkCertificateProvisionFailed(tc.cert)
			default:
				r.MarkCertificateNotReady(tc.cert)
			}

			if err := apistest.CheckCondition(r, RouteConditionCertificateProvisioned, tc.status); err != nil {
				t.Error(err)
			}

			certMessage := r.Status.GetCondition(apis.ConditionReady).Message
			if !strings.Contains(certMessage, tc.wantMessage) {
				t.Errorf("Literal %q not found in status message: %q", tc.wantMessage, certMessage)
			}
		})
	}
}

func TestRouteNotOwnCertificate(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkCertificateNotOwned("cert")

	apistest.CheckConditionFailed(r, RouteConditionCertificateProvisioned, t)
}

func TestEndpointNotOwned(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkEndpointNotOwned("endpoint")

	apistest.CheckConditionFailed(r, RouteConditionIngressReady, t)
}

func TestRouteExternalDomainTLSNotEnabled(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkTLSNotEnabled(ExternalDomainTLSNotEnabledMessage)

	apistest.CheckConditionSucceeded(r, RouteConditionCertificateProvisioned, t)
}

func TestRouteHTTPDowngrade(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkHTTPDowngrade("cert")

	apistest.CheckConditionSucceeded(r, RouteConditionCertificateProvisioned, t)
}

func TestIngressNotConfigured(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkIngressNotConfigured()

	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
}

func TestMarkInRollout(t *testing.T) {
	r := &RouteStatus{}
	r.InitializeConditions()
	r.MarkIngressRolloutInProgress()

	apistest.CheckConditionOngoing(r, RouteConditionIngressReady, t)
}

func TestRolloutDuration(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want time.Duration
	}{{
		name: "empty",
		val:  "",
		want: 0,
	}, {
		name: "invalid",
		val:  "not-a-duration",
		want: 0,
	}, {
		name: "duration",
		val:  "2m1982s",
		want: 2*time.Minute + 1982*time.Second,
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Route{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						serving.RolloutDurationKey: tc.val,
					},
				},
			}
			if got, want := r.RolloutDuration(), tc.want; got != want {
				t.Errorf("RolloutDuration = %v, want: %v", got, want)
			}
		})
	}
}
