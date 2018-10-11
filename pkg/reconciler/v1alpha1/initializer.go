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
	"time"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/tracker"
	kubescheme "github.com/knative/serving/pkg/client/clientset/versioned/scheme"
	servingscheme "github.com/knative/serving/pkg/client/clientset/versioned/scheme"
	"github.com/knative/serving/pkg/reconciler"
	"go.uber.org/zap"
	kubeapicorev1 "k8s.io/api/core/v1"
	kubetypedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func init() {
	// Add serving types to the default Kubernetes Scheme so Events can be
	// logged for serving types.
	servingscheme.AddToScheme(kubescheme.Scheme)
}

type newReconciler func(reconciler.CommonOptions, *DependencyFactory) reconciler.Reconciler

func NewController(
	logger *zap.SugaredLogger,
	newFunc newReconciler,
	componentName string,
	queueName string,
	watcher configmap.Watcher,
	deps *DependencyFactory,
	trackerLease time.Duration,
) (*controller.Impl, error) {

	// There's a circular construction problem between
	// the reconciler and the host controller
	//
	// controller -> reconciler -> tracker -> controller
	//
	// We'll set the reconciler after it's been constructed
	c := controller.NewImpl(nil, logger, queueName)

	logger = logger.Named(componentName).
		With(logkey.ControllerType, componentName)

	recorder := newRecorder(componentName, logger, deps)
	tracker := tracker.New(c.EnqueueKey, trackerLease)

	reconciler := newFunc(reconciler.CommonOptions{
		Logger:        logger,
		Recorder:      recorder,
		ObjectTracker: tracker,
	}, deps)

	if err := setupTriggers(c, tracker, reconciler.Triggers(), deps); err != nil {
		return nil, err
	}

	for _, phase := range reconciler.Phases() {
		if err := setupTriggers(c, tracker, phase.Triggers(), deps); err != nil {
			return nil, err
		}
	}

	if reconciler.ConfigStore() != nil {
		reconciler.ConfigStore().WatchConfigs(watcher)
	}

	c.Reconciler = reconciler

	return c, nil
}

func setupTriggers(
	c *controller.Impl,
	tracker tracker.Interface,
	triggers []reconciler.Trigger,
	deps *DependencyFactory,
) error {

	for _, trigger := range triggers {
		informer, err := deps.InformerFor(trigger.ObjectKind)
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

func newRecorder(
	name string,
	logger *zap.SugaredLogger,
	factory *DependencyFactory,
) record.EventRecorder {

	// Create event broadcaster
	logger.Debugf("Creating event broadcaster for %q", name)

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logger.Named("event-broadcaster").Infof)
	broadcaster.StartRecordingToSink(&kubetypedcorev1.EventSinkImpl{
		Interface: factory.Kubernetes.Client.CoreV1().Events(""),
	})

	return broadcaster.NewRecorder(
		kubescheme.Scheme,
		kubeapicorev1.EventSource{Component: name},
	)
}
