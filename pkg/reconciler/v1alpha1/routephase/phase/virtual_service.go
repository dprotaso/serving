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
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knative/pkg/apis/istio/v1alpha3"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	istiolisters "github.com/knative/pkg/client/listers/istio/v1alpha3"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/tracker"
	"github.com/knative/serving/pkg/apis/serving"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	servingclientsetv1alpha1 "github.com/knative/serving/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	servinglisters "github.com/knative/serving/pkg/client/listers/serving/v1alpha1"
	"github.com/knative/serving/pkg/reconciler"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/resources"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/route/traffic"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
)

type VirtualService struct {
	ConfigurationLister  servinglisters.ConfigurationLister
	RevisionLister       servinglisters.RevisionLister
	VirtualServiceLister istiolisters.VirtualServiceLister

	ServingClient servingclientset.Interface
	SharedClient  sharedclientset.Interface

	Recorder record.EventRecorder
	Tracker  tracker.Interface
}

func (p *VirtualService) Triggers() []reconciler.Trigger {
	return []reconciler.Trigger{
		{
			ObjectKind:  v1alpha3.SchemeGroupVersion.WithKind("VirtualService"),
			OwnerKind:   v1alpha1.SchemeGroupVersion.WithKind("Route"),
			EnqueueType: reconciler.EnqueueOwner,
		}, {
			ObjectKind:  v1alpha1.SchemeGroupVersion.WithKind("Configuration"),
			EnqueueType: reconciler.EnqueueTracker,
		}, {
			ObjectKind:  v1alpha1.SchemeGroupVersion.WithKind("Revision"),
			EnqueueType: reconciler.EnqueueTracker,
		},
	}
}

func (p *VirtualService) Reconcile(ctx context.Context, route *v1alpha1.Route) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling route - virtual service")

	t, err := traffic.BuildTrafficConfiguration(p.ConfigurationLister, p.RevisionLister, route)

	if t != nil {
		// Tell our trackers to reconcile Route whenever the things referred to by our
		// Traffic stanza change.
		for _, configuration := range t.Configurations {
			if err := p.Tracker.Track(objectRef(configuration), route); err != nil {
				return err
			}
		}
		for _, revision := range t.Revisions {
			if revision.Status.IsActivationRequired() {
				logger.Infof("Revision %s/%s is inactive", revision.Namespace, revision.Name)
			}
			if err := p.Tracker.Track(objectRef(revision), route); err != nil {
				return err
			}
		}
	}

	badTarget, isTargetError := err.(traffic.TargetError)
	if err != nil && !isTargetError {
		// An error that's not due to missing traffic target should
		// make us fail fast.
		route.Status.MarkUnknownTrafficError(err.Error())
		return err
	}
	// If the only errors are missing traffic target, we need to
	// update the labels first, so that when these targets recover we
	// receive an update.
	if err := p.syncLabels(logger, route, t); err != nil {
		return err
	}
	if badTarget != nil && isTargetError {
		badTarget.MarkBadTrafficTarget(&route.Status)

		// Traffic targets aren't ready, no need to configure Route.
		return nil
	}
	logger.Info("All referred targets are routable.  Creating Istio VirtualService.")
	if err := p.reconcileService(logger, route, resources.MakeVirtualService(route, t)); err != nil {
		return err
	}
	logger.Info("VirtualService created, marking AllTrafficAssigned with traffic information.")
	route.Status.Traffic = t.GetRevisionTrafficTargets()
	route.Status.MarkTrafficAssigned()
	return nil
}

func (p *VirtualService) reconcileService(
	logger *zap.SugaredLogger,
	route *v1alpha1.Route,
	desiredVirtualService *v1alpha3.VirtualService,
) error {

	ns := desiredVirtualService.Namespace
	name := desiredVirtualService.Name

	virtualService, err := p.VirtualServiceLister.VirtualServices(ns).Get(name)
	if apierrs.IsNotFound(err) {
		virtualService, err = p.SharedClient.NetworkingV1alpha3().VirtualServices(ns).Create(desiredVirtualService)
		if err != nil {
			logger.Error("Failed to create VirtualService", zap.Error(err))
			p.Recorder.Eventf(route, corev1.EventTypeWarning, "CreationFailed",
				"Failed to create VirtualService %q: %v", name, err)
			return err
		}
		p.Recorder.Eventf(route, corev1.EventTypeNormal, "Created",
			"Created VirtualService %q", desiredVirtualService.Name)
	} else if err != nil {
		return err
	} else if !equality.Semantic.DeepEqual(virtualService.Spec, desiredVirtualService.Spec) {
		virtualService.Spec = desiredVirtualService.Spec
		virtualService, err = p.SharedClient.NetworkingV1alpha3().VirtualServices(ns).Update(virtualService)
		if err != nil {
			logger.Error("Failed to update VirtualService", zap.Error(err))
			return err
		}
	}

	// TODO(mattmoor): This is where we'd look at the state of the VirtualService and
	// reflect any necessary state into the Route.
	return err
}

func (c *VirtualService) syncLabels(logger *zap.SugaredLogger, r *v1alpha1.Route, tc *traffic.TrafficConfig) error {
	if err := c.deleteLabelForOutsideOfGivenConfigurations(logger, r, tc.Configurations); err != nil {
		return err
	}
	if err := c.setLabelForGivenConfigurations(logger, r, tc.Configurations); err != nil {
		return err
	}
	return nil
}

func (p *VirtualService) setLabelForGivenConfigurations(
	logger *zap.SugaredLogger,
	route *v1alpha1.Route,
	configMap map[string]*v1alpha1.Configuration,
) error {

	configClient := p.ServingClient.ServingV1alpha1().Configurations(route.Namespace)

	names := []string{}

	// Validate
	for _, config := range configMap {
		names = append(names, config.Name)
		routeName, ok := config.Labels[serving.RouteLabelKey]
		if !ok {
			continue
		}
		// TODO(yanweiguo): add a condition in status for this error
		if routeName != route.Name {
			errMsg := fmt.Sprintf("Configuration %q is already in use by %q, and cannot be used by %q",
				config.Name, routeName, route.Name)
			p.Recorder.Event(route, corev1.EventTypeWarning, "ConfigurationInUse", errMsg)
			logger.Error(errMsg)
			return errors.New(errMsg)
		}
	}
	// Sort the names to give things a deterministic ordering.
	sort.Strings(names)

	// Set label for newly added configurations as traffic target.
	for _, configName := range names {
		config := configMap[configName]
		if config.Labels == nil {
			config.Labels = make(map[string]string)
		} else if _, ok := config.Labels[serving.RouteLabelKey]; ok {
			continue
		}

		if err := setRouteLabelForConfiguration(configClient, config.Name, config.ResourceVersion, &route.Name); err != nil {
			logger.Errorf("Failed to add route label to configuration %q: %s", config.Name, err)
			return err
		}
	}

	return nil
}

func (p *VirtualService) deleteLabelForOutsideOfGivenConfigurations(
	logger *zap.SugaredLogger,
	route *v1alpha1.Route,
	configMap map[string]*v1alpha1.Configuration,
) error {

	ns := route.Namespace

	// Get Configurations set as traffic target before this sync.
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", serving.RouteLabelKey, route.Name))
	if err != nil {
		return err
	}
	oldConfigsList, err := p.ConfigurationLister.Configurations(ns).List(selector)
	if err != nil {
		logger.Errorf("Failed to fetch configurations with label '%s=%s': %s",
			serving.RouteLabelKey, route.Name, err)
		return err
	}

	// Delete label for newly removed configurations as traffic target.
	for _, config := range oldConfigsList {
		if _, ok := configMap[config.Name]; !ok {

			delete(config.Labels, serving.RouteLabelKey)

			configClient := p.ServingClient.ServingV1alpha1().Configurations(config.Namespace)
			if err := setRouteLabelForConfiguration(configClient, config.Name, config.ResourceVersion, nil); err != nil {
				logger.Errorf("Failed to remove route label to configuration %q: %s", config.Name, err)
				return err
			}
		}
	}

	return nil
}

func setRouteLabelForConfiguration(
	configClient servingclientsetv1alpha1.ConfigurationInterface,
	configName string,
	configVersion string,
	routeName *string, // a nil route name will cause the route label to be deleted
) error {

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				serving.RouteLabelKey: routeName,
			},
			"resourceVersion": configVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = configClient.Patch(configName, types.MergePatchType, patch)
	return err

	return nil
}

func (p *VirtualService) InjectConfigurationLister(l servinglisters.ConfigurationLister) {
	p.ConfigurationLister = l
}

func (p *VirtualService) InjectRevisionLister(l servinglisters.RevisionLister) {
	p.RevisionLister = l
}

func (p *VirtualService) InjectVirtualServiceLister(l istiolisters.VirtualServiceLister) {
	p.VirtualServiceLister = l
}

func (p *VirtualService) InjectServingClient(c servingclientset.Interface) {
	p.ServingClient = c
}

func (p *VirtualService) InjectSharedClient(c sharedclientset.Interface) {
	p.SharedClient = c
}

func (p *VirtualService) InjectEventRecorder(r record.EventRecorder) {
	p.Recorder = r
}

func (p *VirtualService) InjectObjectTracker(t tracker.Interface) {
	p.Tracker = t
}

type accessor interface {
	GroupVersionKind() schema.GroupVersionKind
	GetNamespace() string
	GetName() string
}

func objectRef(a accessor) corev1.ObjectReference {
	gvk := a.GroupVersionKind()
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	return corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  a.GetNamespace(),
		Name:       a.GetName(),
	}
}
