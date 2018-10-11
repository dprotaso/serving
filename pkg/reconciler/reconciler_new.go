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

package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	pkgapis "github.com/knative/pkg/apis"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicclientset "k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	EnqueueObject  EnqueueType = "object"
	EnqueueOwner   EnqueueType = "owner"
	EnqueueTracker EnqueueType = "tracker"
)

type (
	EnqueueType string

	Phase interface {
		Triggers() []Trigger
	}

	// Reconciler is the interface that controller implementations are expected
	// to implement, so that the shared controller.Impl can drive work through it.
	Reconciler interface {
		Reconcile(ctx context.Context, key string) error
		Phases() []Phase
		Triggers() []Trigger
		ConfigStore() ConfigStore
	}

	Trigger struct {
		ObjectKind  schema.GroupVersionKind
		OwnerKind   schema.GroupVersionKind
		EnqueueType EnqueueType
	}

	ConfigStore interface {
		ToContext(context.Context) context.Context
		WatchConfigs(configmap.Watcher)
	}

	CommonOptions struct {
		Logger        *zap.SugaredLogger
		Recorder      record.EventRecorder
		ObjectTracker tracker.Interface
	}

	DependencyFactory struct {
		Kubernetes struct {
			Client          kubeclientset.Interface
			InformerFactory kubeinformers.SharedInformerFactory
		}

		Dynamic struct {
			Client dynamicclientset.Interface
		}

		Shared struct {
			Client          sharedclientset.Interface
			InformerFactory sharedinformers.SharedInformerFactory
		}
	}
)

func NewDependencyFactory(cfg *rest.Config, resyncPeriod time.Duration) (*DependencyFactory, error) {
	d := &DependencyFactory{}

	var err error

	d.Kubernetes.Client, err = kubeclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %v", err)
	}

	d.Shared.Client, err = sharedclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building shared clientset: %v", err)
	}

	d.Dynamic.Client, err = dynamicclientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building dynamic clientset: %v", err)
	}

	d.Kubernetes.InformerFactory = kubeinformers.NewSharedInformerFactory(
		d.Kubernetes.Client,
		resyncPeriod,
	)

	d.Shared.InformerFactory = sharedinformers.NewSharedInformerFactory(
		d.Shared.Client,
		resyncPeriod,
	)

	return d, nil
}

func (o *DependencyFactory) StartInformers(stopCh <-chan struct{}) {
	o.Kubernetes.InformerFactory.Start(stopCh)
	o.Shared.InformerFactory.Start(stopCh)
}

func (o *DependencyFactory) WaitForInformerCacheSync(stopCh <-chan struct{}) error {
	waiters := []func(stopCh <-chan struct{}) map[reflect.Type]bool{
		o.Kubernetes.InformerFactory.WaitForCacheSync,
		o.Shared.InformerFactory.WaitForCacheSync,
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
func (d *DependencyFactory) InformerFor(gvk schema.GroupVersionKind) (cache.SharedIndexInformer, error) {
	gvr := pkgapis.KindToResource(gvk)

	if i, err := d.Kubernetes.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	if i, err := d.Shared.InformerFactory.ForResource(gvr); i != nil && err == nil {
		return i.Informer(), nil
	}

	return nil, fmt.Errorf("Unabled to find informer for resource %q", gvr)
}
