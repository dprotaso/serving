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
	"context"
	"testing"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/config"

	. "github.com/knative/serving/pkg/reconciler/v1alpha1/testing"
)

func TestDomainReconcile(t *testing.T) {
	scenarios := PhaseTests{{
		Name: "first-reconcile",
		Context: contextWithDomainConfig(&config.Domain{
			Domains: map[string]*config.LabelSelector{
				"example.com": {},
			}},
		),
		Resource:       simpleRunLatest("default", "first-reconcile", "config"),
		ExpectedStatus: DomainStatus("first-reconcile.default.example.com"),
	}, {
		Name: "config-change",
		Context: contextWithDomainConfig(&config.Domain{
			Domains: map[string]*config.LabelSelector{
				"new-example.com": {},
			}},
		),
		Resource:       simpleRunLatest("default", "config-change", "config"),
		ExpectedStatus: DomainStatus("config-change.default.new-example.com"),
	}, {
		Name: "explicit-route-label-uses-different-domain",
		Context: contextWithDomainConfig(&config.Domain{
			Domains: map[string]*config.LabelSelector{
				"new-example.com": {},
				"explicit-example.com": {
					Selector: map[string]string{"app": "prod"},
				},
			}},
		),
		Resource: addRouteLabel(
			simpleRunLatest("default", "explicit-label", "config"),
			"app", "prod",
		),
		ExpectedStatus: DomainStatus("explicit-label.default.explicit-example.com"),
	}}

	scenarios.Run(t, PhaseSetup(NewDomain))
}

func contextWithDefaultDomain(domain string) context.Context {
	return contextWithDomainConfig(&config.Domain{
		Domains: map[string]*config.LabelSelector{
			domain: {},
		}},
	)
}

func contextWithDomainConfig(c *config.Domain) context.Context {
	return config.ToContext(context.TODO(), &config.Config{
		Domain: c,
	})
}

func addRouteLabel(route *v1alpha1.Route, key, value string) *v1alpha1.Route {
	if route.Labels == nil {
		route.Labels = make(map[string]string)
	}
	route.Labels[key] = value
	return route
}
