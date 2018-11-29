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
	"fmt"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler"
	reconcilerv1alpha1 "github.com/knative/serving/pkg/reconciler/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/config"
)

type (
	Domain struct{}

	DomainStatus string
)

func NewDomain(reconciler.CommonOptions, *reconcilerv1alpha1.DependencyFactory) *Domain {
	return &Domain{}
}

func (s DomainStatus) MergeInto(status *v1alpha1.RouteStatus) error {
	status.Domain = string(s)
	return nil
}

func (p *Domain) Reconcile(ctx context.Context, route *v1alpha1.Route) (DomainStatus, error) {
	return DomainStatus(routeDomain(ctx, route)), nil
}

// TODO(dprotaso) should we just consolidate this with virtual service reconciler?
// My argument is no since we test config changes here
func routeDomain(ctx context.Context, route *v1alpha1.Route) string {
	cfg := config.FromContext(ctx)
	domain := cfg.Domain.LookupDomainForLabels(route.ObjectMeta.Labels)
	return fmt.Sprintf("%s.%s.%s", route.Name, route.Namespace, domain)
}
