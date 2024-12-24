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
	"log"
	"strings"

	"gopkg.in/yaml.v3"
)

func lookupNode(node *yaml.Node, path string) *yaml.Node {
	segments := strings.Split(path, ".")

outer:
	for _, segment := range segments {
		if node.Kind != yaml.MappingNode {
			log.Panicf("node at segment %q not a mapping node\n", segment)
		}

		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if keyNode.Value == segment {
				node = valueNode
				continue outer
			}
		}

		return nil
	}

	return node
}

func lookupString(node *yaml.Node, path string) string {
	if path != "" {
		node = lookupNode(node, path)
	}

	if node == nil {
		log.Panicf("node at path %q not found\n", path)
	}

	if node.Kind != yaml.ScalarNode {
		log.Panicf("node at path %q not a scalar node\n", path)
	}

	if node.ShortTag() != "!!str" {
		log.Panicf("node at path %q not a string node\n", path)
	}

	return node.Value
}

func lookupArray(node *yaml.Node, path string) []*yaml.Node {
	node = lookupNode(node, path)

	if node == nil {
		return nil
	}

	if node.Kind != yaml.SequenceNode {
		log.Panicf("node at path %q not a sequence node\n", path)
	}

	return node.Content
}

func setArray(node *yaml.Node, path string, values []*yaml.Node) {
	node = lookupNode(node, path)

	if node.Kind != yaml.SequenceNode {
		log.Panicf("node at path %q not a sequence node\n", path)
	}

	node.Content = values
}

func deleteKey(node *yaml.Node, key string) {
	if node.Kind != yaml.MappingNode {
		log.Panicf("node is not mapping node")
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func childProperties(node *yaml.Node) *yaml.Node {
	dataType := dataType(node)

	switch dataType {
	case "object":
		node = properties(node)
	case "array":
		node = properties(items(node))
	default:
		node = nil
	}
	if node == nil {
		log.Panicf("node has no children")
	}
	return node
}

func properties(node *yaml.Node) *yaml.Node {
	return lookupNode(node, "properties")
}

func items(node *yaml.Node) *yaml.Node {
	return lookupNode(node, "items")
}

func dataType(node *yaml.Node) string {
	return lookupString(lookupNode(node, "type"), "")
}
