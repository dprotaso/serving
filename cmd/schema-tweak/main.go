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
	"bytes"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"
)

func main() {
	filename := "config/core/300-resources/configuration.yaml"
	fbytes, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalln("failed to read CRD", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(fbytes, &root); err != nil {
		log.Fatalln("failed to unmarshal CRD", err)
	}

	buf := bytes.Buffer{}

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	// document node => content[0] => crd map
	applyOverrides(root.Content[0])

	if err = enc.Encode(&root); err != nil {
		log.Fatalln("failed to marshal CRD", err)
	}

	if err = os.WriteFile(filename, buf.Bytes(), 0700); err != nil {
		log.Fatalln("failed to write CRD", err)
	}
}

func applyOverrides(root *yaml.Node) {
	for _, override := range overrides {
		crdName := lookupString(root, "metadata.name")

		if crdName != override.crdName {
			continue
		}

		versions := lookupArray(root, "spec.versions")

		for _, version := range versions {
			for _, entry := range override.entries {
				schemaNode := lookupNode(version, "schema.openAPIV3Schema")
				applyOverrideEntry(schemaNode, entry)
			}
		}
	}
}

func applyOverrideEntry(schema *yaml.Node, entry entry) {
	node := schema

	segments := strings.Split(entry.path, ".")

	for i := 0; i < len(segments); i++ {
		node = childProperties(node)
		node = lookupNode(node, segments[i])

		if node == nil {
			log.Fatalf("node at path %q not found\n", entry.path)
		}
	}

	if node.Kind != yaml.MappingNode {
		log.Fatalf("node at path %q not a mapping node\n", entry.path)
	}

	// drop required fields
	dropRequiredFields(node, entry.dropRequired)
}

func dropRequiredFields(node *yaml.Node, fields sets.Set[string]) {
	dataType := dataType(node)

	switch dataType {
	case "array":
		node = items(node)
	}

	required := lookupArray(node, "required")

	if len(required) == 0 {
		deleteKey(node, "required")
		return
	}

	for i := 0; i < len(required); i++ {
		if fields.Has(required[i].Value) {
			required = append(required[:i], required[i+1:]...)
			break
		}
	}

	if len(required) == 0 {
		deleteKey(node, "required")
	} else {
		setArray(node, "required", required)
	}
}
