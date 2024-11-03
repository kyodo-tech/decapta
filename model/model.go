// Copyright 2024 Kyodo Tech合同会社
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	decaptaIDField = "slug"
	decaptaPrefix  = "decapta_"
)

func reservedFields() map[string]bool {
	return map[string]bool{
		"data": true,
	}
}

type Collection struct {
	Name            string  `yaml:"name"`
	Label           string  `yaml:"label"`
	Folder          string  `yaml:"folder,omitempty"`
	Create          bool    `yaml:"create,omitempty"`
	Slug            string  `yaml:"slug,omitempty"`
	IdentifierField string  `yaml:"identifier_field,omitempty"`
	Format          string  `yaml:"format,omitempty"`
	Extension       string  `yaml:"extension,omitempty"`
	Editor          Editor  `yaml:"editor,omitempty"`
	Files           []File  `yaml:"files,omitempty"`
	Fields          []Field `yaml:"fields,omitempty"`
}

type Editor struct {
	Preview bool `yaml:"preview,omitempty"`
}

type File struct {
	Name   string  `yaml:"name"`
	Label  string  `yaml:"label"`
	File   string  `yaml:"file"`
	Fields []Field `yaml:"fields"`
}

type Field struct {
	Label       string                 `yaml:"label"`
	Name        string                 `yaml:"name"`
	Widget      string                 `yaml:"widget"`
	Modes       []string               `yaml:"modes,omitempty"`
	Fields      []Field                `yaml:"fields,omitempty"`
	Collapsed   bool                   `yaml:"collapsed,omitempty"`
	Hint        string                 `yaml:"hint,omitempty"`
	Placeholder string                 `yaml:"placeholder,omitempty"`
	Required    bool                   `yaml:"required,omitempty"`
	Default     interface{}            `yaml:"default,omitempty"`
	Pattern     string                 `yaml:"pattern,omitempty"`
	PatternMsg  string                 `yaml:"pattern_msg,omitempty"`
	Meta        map[string]interface{} `yaml:"meta,omitempty"`
}

func writeCollections(collections []Collection, templateContent, indexHTML []byte, outputFile string) error {
	var rootNode yaml.Node

	// Check if config.yml exists
	if _, err := os.Stat(outputFile); err == nil {
		// Load existing config.yml with comment preservation
		configData, err := os.ReadFile(outputFile)
		if err != nil {
			return fmt.Errorf("error reading existing config.yml: %v", err)
		}
		err = yaml.Unmarshal(configData, &rootNode)
		if err != nil {
			return fmt.Errorf("error parsing existing config.yml: %v", err)
		}
	} else {
		// Parse the template file instead if no config.yml exists
		err := yaml.Unmarshal(templateContent, &rootNode)
		if err != nil {
			return fmt.Errorf("error parsing template content: %v", err)
		}
	}

	// Find or add the collections node within rootNode
	collectionsNode := findOrCreateCollectionsNode(&rootNode)
	upsertCollections(collectionsNode, collections)

	// Marshal updated rootNode to YAML while preserving comments
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	err := enc.Encode(&rootNode)
	if err != nil {
		return fmt.Errorf("error generating final YAML: %v", err)
	}

	// Write final config with preserved comments and structure
	err = os.WriteFile(outputFile, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error writing config.yml: %v", err)
	}

	// Write index.html file
	basePath := filepath.Dir(outputFile)
	err = os.MkdirAll(basePath, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}
	err = os.WriteFile(filepath.Join(basePath, "index.html"), indexHTML, 0644)
	if err != nil {
		return fmt.Errorf("error writing index.html: %v", err)
	}

	return nil
}

func findOrCreateCollectionsNode(rootNode *yaml.Node) *yaml.Node {
	for i := 0; i < len(rootNode.Content[0].Content); i += 2 {
		keyNode := rootNode.Content[0].Content[i]
		if keyNode.Value == "collections" {
			return rootNode.Content[0].Content[i+1]
		}
	}

	// If collections node doesn't exist, create and append it
	collectionsNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	rootNode.Content[0].Content = append(rootNode.Content[0].Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "collections"}, collectionsNode)
	return collectionsNode
}

func upsertCollections(collectionsNode *yaml.Node, collections []Collection) {
	for _, newColl := range collections {
		upserted := false
		for _, existingNode := range collectionsNode.Content {
			existingCollection := Collection{}
			_ = existingNode.Decode(&existingCollection)

			// Match by Folder path for upsert
			if existingCollection.Folder == newColl.Folder {
				// Update only missing fields without overwriting existing configurations
				mergeCollectionFields(existingNode, newColl)
				upserted = true
				break
			}
		}
		// If no match was found, append the new collection
		if !upserted {
			newCollectionNode := yaml.Node{}
			_ = newCollectionNode.Encode(newColl)
			collectionsNode.Content = append(collectionsNode.Content, &newCollectionNode)
		}
	}
}

func mergeCollectionFields(existingNode *yaml.Node, newColl Collection) {
	var tempNode yaml.Node
	_ = tempNode.Encode(newColl)

	idFieldValue := findFieldInNode(existingNode, decaptaIDField)
	if idFieldValue == nil && newColl.IdentifierField != "" {
		existingNode.Content = append(existingNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: decaptaIDField}, &yaml.Node{Kind: yaml.ScalarNode, Value: newColl.IdentifierField})
	}

	// Iterate over newColl fields and upsert only missing fields
	for i := 0; i < len(tempNode.Content[0].Content); i += 2 {
		key := tempNode.Content[0].Content[i].Value
		value := tempNode.Content[0].Content[i+1]

		existingValue := findFieldInNode(existingNode, key)
		if existingValue == nil {
			// Field doesn't exist in the existing node, so add it
			existingNode.Content = append(existingNode.Content, tempNode.Content[0].Content[i], value)
		} else if key == "fields" {
			// Recursively merge fields if the key is "fields"
			mergeFields(existingValue, value)
		} else if key == "editor" {
			// Recursively merge editor configuration
			mergeEditor(existingValue, value)
		}
	}

}

func findFieldInNode(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func mergeFields(existingFieldsNode, newFieldsNode *yaml.Node) {
	for _, newFieldNode := range newFieldsNode.Content {
		fieldName := findFieldInNode(newFieldNode, "name").Value
		existingFieldNode := findFieldByName(existingFieldsNode, fieldName)
		if existingFieldNode == nil {
			// Field doesn't exist, so add it
			existingFieldsNode.Content = append(existingFieldsNode.Content, newFieldNode)
		} else {
			// If field exists, no overwrite occurs, preserving manual changes
			continue
		}
	}
}

func findFieldByName(fieldsNode *yaml.Node, fieldName string) *yaml.Node {
	for _, fieldNode := range fieldsNode.Content {
		if nameNode := findFieldInNode(fieldNode, "name"); nameNode != nil && nameNode.Value == fieldName {
			return fieldNode
		}
	}
	return nil
}

func mergeEditor(existingEditorNode, newEditorNode *yaml.Node) {
	for i := 0; i < len(newEditorNode.Content); i += 2 {
		key := newEditorNode.Content[i].Value
		value := newEditorNode.Content[i+1]
		existingValue := findFieldInNode(existingEditorNode, key)
		if existingValue == nil {
			existingEditorNode.Content = append(existingEditorNode.Content, newEditorNode.Content[i], value)
		}
	}
}
