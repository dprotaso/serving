package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	applycore "k8s.io/client-go/applyconfigurations/core/v1"
)

func main() {
	rev := v1.Revision{}
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(revYaml), 4096)
	decoder.Decode(&rev)

	bytes, err := json.Marshal(rev.Spec.PodSpec)
	if err != nil {
		panic(err)
	}

	spec := applycore.PodSpec()

	err = json.Unmarshal(bytes, spec)
	if err != nil {
		panic(err)
	}

	// rewrite Rev to a Pod to extract the spec
	// pod := &corev1.Pod{ObjectMeta: rev.ObjectMeta, Spec: rev.Spec.PodSpec}
	// for i := range rev.ObjectMeta.ManagedFields {
	// 	rev.ObjectMeta.ManagedFields[i].Operation = metav1.ManagedFieldsOperationApply
	// }
	// podApply, err := applycore.ExtractPod(pod, "controller")
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Println(cmp.Diff("", spec))

}

func ExtractInto(object runtime.Object, objectType typed.ParseableType, fieldManager string, applyConfiguration interface{}, subresource string) error {
	typedObj, err := toTyped(object, objectType)
	if err != nil {
		return fmt.Errorf("error converting obj to typed: %w", err)
	}

	accessor, err := meta.Accessor(object)
	if err != nil {
		return fmt.Errorf("error accessing metadata: %w", err)
	}
	fieldsEntry, ok := findManagedFields(accessor, fieldManager, subresource)
	if !ok {
		return nil
	}
	fieldset := &fieldpath.Set{}
	err = fieldset.FromJSON(bytes.NewReader(fieldsEntry.FieldsV1.Raw))
	if err != nil {
		return fmt.Errorf("error marshalling FieldsV1 to JSON: %w", err)
	}

	u := typedObj.ExtractItems(fieldset.Leaves()).AsValue().Unstructured()
	m, ok := u.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to convert managed fields for %s to unstructured, expected map, got %T", fieldManager, u)
	}

	// set the type meta manually if it doesn't exist to avoid missing kind errors
	// when decoding from unstructured JSON
	if _, ok := m["kind"]; !ok && object.GetObjectKind().GroupVersionKind().Kind != "" {
		m["kind"] = object.GetObjectKind().GroupVersionKind().Kind
		m["apiVersion"] = object.GetObjectKind().GroupVersionKind().GroupVersion().String()
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, applyConfiguration); err != nil {
		return fmt.Errorf("error extracting into obj from unstructured: %w", err)
	}
	return nil
}

func findManagedFields(accessor metav1.Object, fieldManager string, subresource string) (metav1.ManagedFieldsEntry, bool) {
	objManagedFields := accessor.GetManagedFields()
	for _, mf := range objManagedFields {
		if mf.Manager == fieldManager && mf.Operation == metav1.ManagedFieldsOperationApply && mf.Subresource == subresource {
			return mf, true
		}
	}
	return metav1.ManagedFieldsEntry{}, false
}

func toTyped(obj runtime.Object, objectType typed.ParseableType) (*typed.TypedValue, error) {
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		return objectType.FromUnstructured(o.Object)
	default:
		return objectType.FromStructured(o)
	}
}

var revYaml = `
apiVersion: serving.knative.dev/v1
kind: Revision
metadata:
  annotations:
    client.knative.dev/updateTimestamp: "2023-10-02T18:38:57Z"
    client.knative.dev/user-image: ghcr.io/knative/helloworld-go:latest
    serving.knative.dev/creator: kubernetes-admin
    serving.knative.dev/routes: hello
    serving.knative.dev/routingStateModified: "2023-10-02T18:38:57Z"
  creationTimestamp: "2023-10-02T18:38:57Z"
  generation: 1
  labels:
    serving.knative.dev/configuration: hello
    serving.knative.dev/configurationGeneration: "1"
    serving.knative.dev/configurationUID: 2ae2a4d8-32ce-40de-beae-92408621e186
    serving.knative.dev/routingState: active
    serving.knative.dev/service: hello
    serving.knative.dev/serviceUID: f17aaf0f-bd32-4122-a787-3ed829d59042
  managedFields:
  - apiVersion: serving.knative.dev/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .: {}
          f:client.knative.dev/updateTimestamp: {}
          f:client.knative.dev/user-image: {}
          f:serving.knative.dev/creator: {}
          f:serving.knative.dev/routes: {}
          f:serving.knative.dev/routingStateModified: {}
        f:labels:
          .: {}
          f:serving.knative.dev/configuration: {}
          f:serving.knative.dev/configurationGeneration: {}
          f:serving.knative.dev/configurationUID: {}
          f:serving.knative.dev/routingState: {}
          f:serving.knative.dev/service: {}
          f:serving.knative.dev/serviceUID: {}
        f:ownerReferences:
          .: {}
          k:{"uid":"2ae2a4d8-32ce-40de-beae-92408621e186"}: {}
      f:spec:
        .: {}
        f:containerConcurrency: {}
        f:containers: {}
        f:enableServiceLinks: {}
        f:timeoutSeconds: {}
    manager: controller
    operation: Update
    time: "2023-10-02T18:38:57Z"
  - apiVersion: serving.knative.dev/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:status:
        .: {}
        f:conditions: {}
        f:observedGeneration: {}
    manager: controller
    operation: Update
    subresource: status
    time: "2023-10-02T18:38:57Z"
  name: hello-00001
  namespace: default
  ownerReferences:
  - apiVersion: serving.knative.dev/v1
    blockOwnerDeletion: true
    controller: true
    kind: Configuration
    name: hello
    uid: 2ae2a4d8-32ce-40de-beae-92408621e186
  resourceVersion: "70860"
  uid: 285cebdf-6d9a-4799-ab64-8133a71f2596
spec:
  containerConcurrency: 0
  containers:
  - env:
    - name: TARGET
      value: World
    image: ghcr.io/knative/helloworld-go:latest
    name: user-container
    ports:
    - containerPort: 8080
      protocol: TCP
    readinessProbe:
      successThreshold: 1
      tcpSocket:
        port: 0
    resources: {}
  enableServiceLinks: false
  timeoutSeconds: 300
status:
  conditions:
  - lastTransitionTime: "2023-10-02T18:38:57Z"
    status: Unknown
    type: ContainerHealthy
  - lastTransitionTime: "2023-10-02T18:38:57Z"
    reason: ResolvingDigests
    status: Unknown
    type: Ready
  - lastTransitionTime: "2023-10-02T18:38:57Z"
    reason: ResolvingDigests
    status: Unknown
    type: ResourcesAvailable
  observedGeneration: 1
`
