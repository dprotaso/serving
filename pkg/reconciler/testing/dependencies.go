package testing

import (
	"github.com/knative/pkg/apis"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	fakesharedclientset "github.com/knative/pkg/client/clientset/versioned/fake"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"
	"github.com/knative/serving/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicclientset "k8s.io/client-go/dynamic"
	fakedynamicclientset "k8s.io/client-go/dynamic/fake"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

type AddToScheme func(*runtime.Scheme)

func NewFakeDependencies(objs []runtime.Object, schemes ...AddToScheme) (*runtime.Scheme, ObjectSorter, *reconciler.DependencyFactory) {
	scheme := runtime.NewScheme()
	fakekubeclientset.AddToScheme(scheme)
	fakesharedclientset.AddToScheme(scheme)

	// The dynamic client requires the correct schemes to handle
	// objects that are not unstructured
	for _, addToScheme := range schemes {
		addToScheme(scheme)
	}

	sorter := NewObjectSorter(scheme)
	sorter.AddObjects(objs...)

	k8sObjs := sorter.ObjectsForSchemeFunc(fakekubeclientset.AddToScheme)
	sharedObjs := sorter.ObjectsForSchemeFunc(fakesharedclientset.AddToScheme)

	fakeKubeClientset := fakekubeclientset.NewSimpleClientset(k8sObjs...)
	fakeSharedClientset := fakesharedclientset.NewSimpleClientset(sharedObjs...)

	// Initialize the dynamic client with all the objects
	fakeClient := fakedynamicclientset.NewSimpleDynamicClient(runtime.NewScheme())

	kubeInformer := kubeinformers.NewSharedInformerFactory(fakeKubeClientset, 0)
	sharedInformer := sharedinformers.NewSharedInformerFactory(fakeSharedClientset, 0)

	for _, obj := range objs {
		kinds, _, _ := scheme.ObjectKinds(obj)
		for _, kind := range kinds {
			resource := apis.KindToResource(kind)
			if inf, _ := kubeInformer.ForResource(resource); inf != nil {
				inf.Informer().GetStore().Add(obj)
			}
			if inf, _ := sharedInformer.ForResource(resource); inf != nil {
				inf.Informer().GetStore().Add(obj)
			}
		}
	}

	return scheme, sorter, &reconciler.DependencyFactory{
		Kubernetes: struct {
			Client          kubeclientset.Interface
			InformerFactory kubeinformers.SharedInformerFactory
		}{
			Client:          fakeKubeClientset,
			InformerFactory: kubeInformer,
		},
		Shared: struct {
			Client          sharedclientset.Interface
			InformerFactory sharedinformers.SharedInformerFactory
		}{
			Client:          fakeSharedClientset,
			InformerFactory: sharedInformer,
		},
		Dynamic: struct {
			Client dynamicclientset.Interface
		}{
			Client: fakeClient,
		},
	}
}
