/*
Copyright 2025 The Knative Authors

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

package main

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
)

type override struct {
	crdName string
	entries []entry
}

type entry struct {
	path              string
	description       string
	allowedFields     []string
	featureFlagFields []flagField
	dropRequired      sets.Set[string]

	// Drops list-type
	//   x-kubernetes-list-map-keys:
	//     - name
	//   x-kubernetes-list-type: map
	dropListType bool
}

type flagField struct {
	name string
	flag string
}

var overrides = []override{{
	crdName: "configurations.serving.knative.dev",
	entries: append([]entry{}, podSpecOverrides...),
}}

var podSpecOverrides = []entry{{
	path: "spec.template.spec",
	dropRequired: sets.New(
		"containers",
	),
	allowedFields: append([]string{
		"automountServiceAccountToken",
		"containers",
		"enableServiceLinks",
		"imagePullSecrets",
		"serviceAccountName",
		"volumes",
	}, revisionSpecFields()...),
	featureFlagFields: []flagField{
		{name: "affinity", flag: "kubernetes.podspec-affinity"},
		{name: "dnsConfig", flag: "kubernetes.podspec-dnsconfig"},
		{name: "dnsPolicy", flag: "kubernetes.podspec-dnspolicy"},
		{name: "hostAliases", flag: "kubernetes.podspec-hostaliases"},
		{name: "hostIPC", flag: "kubernetes.podspec-hostipc"},
		{name: "hostNetwork", flag: "kubernetes.podspec-hostnetwork"},
		{name: "hostPID", flag: "kubernetes.podspec-hostpid"},
		{name: "initContainers", flag: "kubernetes.podspec-init-containers"},
		{name: "nodeSelector", flag: "kubernetes.podspec-nodeselector"},
		{name: "priorityClassName", flag: "kubernetes.podspec-priorityclassname"},
		{name: "runtimeClassName", flag: "kubernetes.podspec-runtimeclassname"},
		{name: "schedulerName", flag: "kubernetes.podspec-schedulername"},
		{name: "securityContext", flag: "kubernetes.podspec-securitycontext"},
		{name: "shareProcessNamespace", flag: "kubernetes.podspec-shareprocessnamespace"},
		{name: "tolerations", flag: "kubernetes.podspec-tolerations"},
		{name: "topologySpreadConstraints", flag: "kubernetes.podspec-tolerations"},
	},
}, {
	path:         "spec.template.spec.containers",
	dropListType: true,
	dropRequired: sets.New("name"),
	allowedFields: []string{
		"name",
		"args",
		"command",
		"env",
		"workingDir",
		"envFrom",
		"image",
		"imagePullPolicy",
		"livenessProbe",
		"ports",
		"readinessProbe",
		"resources",
		"securityContext",
		"startupProbe",
		"terminationMessagePath",
		"terminationMessagePolicy",
		"volumeMounts",
	},
}, {
	path:         "spec.template.spec.initContainers",
	dropListType: true,
	dropRequired: sets.New("name"),
}, {
	path:         "spec.template.spec.containers.ports",
	dropListType: true,
	dropRequired: sets.New("containerPort"),
}, {
	path: "spec.template.spec.enableServiceLinks",
	description: "EnableServiceLinks indicates whether information about" +
		"services should be injected into pod's environment variables, " +
		"matching the syntax of Docker links. Optional: Knative defaults this to false.",
}}

func revisionSpecFields() []string {
	var (
		fields  []string
		revType = reflect.TypeOf(v1.RevisionSpec{})
	)

	for i := 0; i < revType.NumField(); i++ {
		if revType.Field(i).Name == "PodSpec" {
			continue
		}

		jsonTag := revType.Field(i).Tag.Get("json")
		fields = append(fields, strings.Split(jsonTag, ",")[0])
	}

	return fields
}
