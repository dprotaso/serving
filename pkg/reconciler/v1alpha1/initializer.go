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

package v1alpha1

import (
	"fmt"
	"reflect"
	"time"

	cachingclientset "github.com/knative/caching/pkg/client/clientset/versioned"
	cachinginformers "github.com/knative/caching/pkg/client/informers/externalversions"
	pkgapis "github.com/knative/pkg/apis"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/tracker"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	kubescheme "github.com/knative/serving/pkg/client/clientset/versioned/scheme"
	servingscheme "github.com/knative/serving/pkg/client/clientset/versioned/scheme"
	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"
	"github.com/knative/serving/pkg/reconciler"
	k8sinject "github.com/knative/serving/pkg/reconciler/inject"
	v1alpha1inject "github.com/knative/serving/pkg/reconciler/v1alpha1/inject"
	"go.uber.org/zap"
	kubeapicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicclientset "k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	kubetypedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func init() {
	// Add serving types to the default Kubernetes Scheme so Events can be
	// logged for serving types.
	servingscheme.AddToScheme(kubescheme.Scheme)
}

type Initializer struct {
	ResyncPeriod time.Duration
	Logger       *zap.SugaredLogger

	Kubernetes struct {
		Client          kubeclientset.Interface
		InformerFactory kubeinformers.SharedInformerFactory
	}

	Shared struct {
		Client          sharedclientset.Interface
		InformerFactory sharedinformers.SharedInformerFactory
	}

	Serving struct {
		Client          servingclientset.Interface
		InformerFactory servinginformers.SharedInformerFactory
	}

	Dynamic struct {
		Client dynamicclientset.Interface
	}

	Caching struct {
		Client          cachingclientset.Interface
		InformerFactory cachinginformers.SharedInformerFactory
	}
}

// Options
type InitOption func(*Initializer) error

func Resync(resync time.Duration) InitOption {
	return func(i *Initializer) error {
		i.ResyncPeriod = resync
		return nil
	}
}

func NewInitializer(
	cfg *restclient.Config,
	logger *zap.SugaredLogger,
	options ...InitOption,
) (*Initializer, error) {

	i := &Initializer{
		ResyncPeriod: 30 * time.Second,
		Logger:       logger,
	}

	var err error

	for _, o := range options {
		o(i)
	}

	i.Kubernetes.Client, err = kubeclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %v", err)
	}

	i.Shared.Client, err = sharedclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building shared clientset: %v", err)
	}

	i.Serving.Client, err = servingclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building serving clientset: %v", err)
	}

	i.Dynamic.Client, err = dynamicclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building build clientset: %v", err)
	}

	i.Caching.Client, err = cachingclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building caching clientset: %v", err)
	}

	i.Kubernetes.InformerFactory = kubeinformers.NewSharedInformerFactory(
		i.Kubernetes.Client,
		i.ResyncPeriod,
	)

	i.Shared.InformerFactory = sharedinformers.NewSharedInformerFactory(
		i.Shared.Client,
		i.ResyncPeriod,
	)

	i.Serving.InformerFactory = servinginformers.NewSharedInformerFactory(
		i.Serving.Client,
		i.ResyncPeriod,
	)

	i.Caching.InformerFactory = cachinginformers.NewSharedInformerFactory(
		i.Caching.Client,
		i.ResyncPeriod,
	)

	return i, nil
}

func (i *Initializer) Start(stopCh <-chan struct{}) {
	i.Kubernetes.InformerFactory.Start(stopCh)
	i.Shared.InformerFactory.Start(stopCh)
	i.Serving.InformerFactory.Start(stopCh)
	i.Caching.InformerFactory.Start(stopCh)
}

func (i *Initializer) WaitForCacheSync(stopCh <-chan struct{}) error {
	waiters := []func(stopCh <-chan struct{}) map[reflect.Type]bool{
		i.Kubernetes.InformerFactory.WaitForCacheSync,
		i.Shared.InformerFactory.WaitForCacheSync,
		i.Serving.InformerFactory.WaitForCacheSync,
		i.Caching.InformerFactory.WaitForCacheSync,
	}

	for _, wait := range waiters {
		result := wait(stopCh)

		for informerType, started := range result {
			if !started {
				return fmt.Errorf("failed to wait for cache sync for type %q", informerType.Name())
			}
		}
	}

	return nil
}

func (i *Initializer) NewController(
	newFunc reconciler.NewFunc,
	componentName string,
	queueName string,
	watcher configmap.Watcher,
) (*controller.Impl, error) {

	logger := i.Logger.Named(componentName).
		With(logkey.ControllerType, componentName)

	recorder := i.newRecorder(componentName, logger)

	reconciler := newFunc(reconciler.Common{
		Logger:   logger,
		Recorder: recorder,
	})

	c := controller.NewImpl(reconciler, logger, queueName)
	tracker := tracker.New(c.EnqueueKey, 3*i.ResyncPeriod)

	i.init(
		c,
		reconciler,
		logger,
		recorder,
		tracker,
	)

	if err := i.setupTriggers(c, tracker, reconciler.Triggers()); err != nil {
		return nil, err
	}

	for _, phase := range reconciler.Phases() {
		if err := i.setupTriggers(c, tracker, phase.Triggers()); err != nil {
			return nil, err
		}
	}

	if reconciler.ConfigStore() != nil {
		reconciler.ConfigStore().WatchConfigs(watcher)
	}

	return c, nil
}

func (i *Initializer) init(
	controller *controller.Impl,
	reconciler reconciler.Reconciler,
	logger *zap.SugaredLogger,
	recorder record.EventRecorder,
	tracker tracker.Interface,
) {

	k8sinject.ObjectTrackerInto(tracker, reconciler)
	i.inject(reconciler)

	for _, phase := range reconciler.Phases() {
		k8sinject.LoggerInto(logger, phase)
		k8sinject.EventRecorderInto(recorder, phase)
		k8sinject.ObjectTrackerInto(tracker, phase)

		i.inject(phase)
	}
}

func (i *Initializer) setupTriggers(
	c *controller.Impl,
	tracker tracker.Interface,
	triggers []reconciler.Trigger,
) error {

	for _, trigger := range triggers {
		informer, err := i.informerFor(trigger.ObjectKind)
		if err != nil {
			return err
		}

		var enqueue func(obj interface{})

		switch trigger.EnqueueType {
		case reconciler.EnqueueObject:
			enqueue = c.Enqueue
			break
		case reconciler.EnqueueOwner:
			enqueue = c.EnqueueControllerOf
		case reconciler.EnqueueTracker:
			enqueue = tracker.OnChanged
		default:
			return fmt.Errorf("Unknown trigger enqueue type %q", trigger.EnqueueType)
		}

		var handler cache.ResourceEventHandler = cache.ResourceEventHandlerFuncs{
			AddFunc:    enqueue,
			UpdateFunc: controller.PassNew(enqueue),
			DeleteFunc: enqueue,
		}

		if !trigger.OwnerKind.Empty() {
			handler = cache.FilteringResourceEventHandler{
				FilterFunc: controller.Filter(trigger.OwnerKind),
				Handler:    handler,
			}
		}

		informer.AddEventHandler(handler)
	}

	return nil
}

func (i *Initializer) informerFor(gvk schema.GroupVersionKind) (cache.SharedIndexInformer, error) {
	gvr := pkgapis.KindToResource(gvk)

	if i, err := i.Kubernetes.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	if i, err := i.Serving.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	if i, err := i.Caching.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	if i, err := i.Shared.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	return nil, fmt.Errorf("Unabled to find informer for resource %q", gvr)
}

func (i *Initializer) inject(obj interface{}) {
	k8sinject.KubeClientInto(i.Kubernetes.Client, obj)
	k8sinject.KubernetesListersInto(i.Kubernetes.InformerFactory, obj)

	v1alpha1inject.ServingClientInto(i.Serving.Client, obj)
	v1alpha1inject.ServingListersInto(i.Serving.InformerFactory, obj)

	v1alpha1inject.SharedClientInto(i.Shared.Client, obj)
	v1alpha1inject.SharedListersInto(i.Shared.InformerFactory, obj)

	v1alpha1inject.CachingClientInto(i.Caching.Client, obj)
	v1alpha1inject.CachingListersInto(i.Caching.InformerFactory, obj)
}

func (i *Initializer) newRecorder(
	name string,
	logger *zap.SugaredLogger,
) record.EventRecorder {

	// Create event broadcaster
	logger.Debugf("Creating event broadcaster for %q", name)

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logger.Named("event-broadcaster").Infof)
	broadcaster.StartRecordingToSink(&kubetypedcorev1.EventSinkImpl{
		Interface: i.Kubernetes.Client.CoreV1().Events(""),
	})

	return broadcaster.NewRecorder(
		kubescheme.Scheme,
		kubeapicorev1.EventSource{Component: name},
	)
}
