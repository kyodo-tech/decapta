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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ARBPreProcess converts ARB files into single content files per language for Decap CMS.
func ARBPreProcess(arbDir string, contentDir string) error {
	files, err := os.ReadDir(arbDir)
	if err != nil {
		return fmt.Errorf("error reading ARB directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".arb") {
			continue
		}

		arbFilePath := filepath.Join(arbDir, file.Name())
		language := extractLanguage(file.Name())
		if language == "" {
			fmt.Printf("Skipping file with unrecognized language code: %s\n", file.Name())
			continue
		}

		arbFile, err := os.ReadFile(arbFilePath)
		if err != nil {
			return fmt.Errorf("error reading ARB file %s: %v", arbFilePath, err)
		}

		// Use OrderedMap to preserve key order
		var arbData OrderedMap
		if err := json.Unmarshal(arbFile, &arbData); err != nil {
			return fmt.Errorf("error parsing ARB file %s: %v", arbFilePath, err)
		}

		// Prepare translation data for front matter
		frontMatter := make(map[string]interface{})

		for _, kv := range arbData {
			key := kv.Key
			value := kv.Value

			if strings.HasPrefix(key, "@") {
				continue // Skip metadata keys for now
			}

			translationEntry := map[string]interface{}{
				"value": value,
			}

			// Handle metadata
			metadataKey := fmt.Sprintf("@%s", key)
			if metadataValue, found := arbData.Get(metadataKey); found {
				translationEntry["metadata"] = metadataValue
			}

			frontMatter[key] = translationEntry
		}

		// Create content directory if it doesn't exist
		err = os.MkdirAll(contentDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error creating content directory %s: %v", contentDir, err)
		}

		// Write to content file per language
		contentFilePath := filepath.Join(contentDir, fmt.Sprintf("%s.yaml", language))
		yamlData, err := yaml.Marshal(frontMatter)
		if err != nil {
			return fmt.Errorf("error marshaling YAML for language %s: %v", language, err)
		}

		err = os.WriteFile(contentFilePath, yamlData, 0644)
		if err != nil {
			return fmt.Errorf("error writing content file %s: %v", contentFilePath, err)
		}
	}

	return nil
}

// ARBPostProcess reads the content files and reconstructs the ARB JSON files, preserving key order.
func ARBPostProcess(contentDir string, arbDir string) error {
	files, err := os.ReadDir(contentDir)
	if err != nil {
		return fmt.Errorf("error reading content directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		language := strings.TrimSuffix(file.Name(), ".yaml")
		contentFilePath := filepath.Join(contentDir, file.Name())

		yamlContent, err := os.ReadFile(contentFilePath)
		if err != nil {
			return fmt.Errorf("error reading content file %s: %v", contentFilePath, err)
		}

		frontMatter, err := YAMLToMapStringInterface(yamlContent)
		if err != nil {
			return fmt.Errorf("error unmarshaling YAML file %s: %v", contentFilePath, err)
		}

		// Reconstruct the ARB data with preserved order
		var arbData OrderedMap

		for _, key := range getOrderedKeys(frontMatter) {
			entry, ok := frontMatter[key].(map[string]interface{})
			if !ok {
				return fmt.Errorf("unexpected data type for key %s in file %s", key, contentFilePath)
			}

			value := entry["value"]
			arbData = append(arbData, KVPair{Key: key, Value: value})

			// Include metadata if present
			if metadata, ok := entry["metadata"]; ok {
				metadataKey := fmt.Sprintf("@%s", key)
				arbData = append(arbData, KVPair{Key: metadataKey, Value: metadata})
			}
		}

		// Convert arbData to JSON with preserved order
		arbJSON, err := arbData.MarshalJSON()
		if err != nil {
			return fmt.Errorf("error marshaling ARB JSON for language %s: %v", language, err)
		}

		// ensure the output directory exists
		err = os.MkdirAll(arbDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error creating output directory %s: %v", arbDir, err)
		}

		arbFilePath := filepath.Join(arbDir, fmt.Sprintf("app_%s.arb", language))
		err = os.WriteFile(arbFilePath, arbJSON, 0644)
		if err != nil {
			return fmt.Errorf("error writing ARB file %s: %v", arbFilePath, err)
		}
	}

	return nil
}

func ARBGenerateConfig(arbDir string, outputFile string, templateData, indexHTML []byte, contentDir string) error {
	files, err := os.ReadDir(arbDir)
	if err != nil {
		return fmt.Errorf("error reading ARB directory: %v", err)
	}

	var collections []Collection

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".arb") {
			continue
		}

		language := extractLanguage(file.Name())
		if language == "" {
			fmt.Printf("Skipping file with unrecognized language code: %s\n", file.Name())
			continue
		}

		collection := Collection{
			Name:  fmt.Sprintf("translations_%s", language),
			Label: fmt.Sprintf("Translations (%s)", strings.ToUpper(language)),
			Files: []File{
				{
					Name:   fmt.Sprintf("translation_%s", language),
					Label:  fmt.Sprintf("Translation (%s)", strings.ToUpper(language)),
					File:   filepath.Join(contentDir, fmt.Sprintf("%s.yaml", language)),
					Fields: []Field{},
				},
			},
		}

		// Read the content file to get the keys for fields
		contentFilePath := filepath.Join(contentDir, fmt.Sprintf("%s.yaml", language))
		yamlContent, err := os.ReadFile(contentFilePath)
		if err != nil {
			return fmt.Errorf("error reading content file %s: %v", contentFilePath, err)
		}

		var frontMatter map[string]interface{}
		err = yaml.Unmarshal(yamlContent, &frontMatter)
		if err != nil {
			return fmt.Errorf("error unmarshaling YAML file %s: %v", contentFilePath, err)
		}

		// Generate fields for each translation key
		for _, key := range getOrderedKeys(frontMatter) {
			entry, ok := frontMatter[key].(map[string]interface{})
			if !ok {
				return fmt.Errorf("unexpected data type for key %s in file %s", key, contentFilePath)
			}

			field := Field{
				Label:  strings.ReplaceAll(key, "_", " "),
				Name:   key,
				Widget: "string",
			}

			// If there are placeholders or metadata, format them properly
			if metadata, ok := entry["metadata"]; ok {
				if placeholders, exists := metadata.(map[string]interface{})["placeholders"]; exists {
					field.Hint = formatPlaceholders(placeholders.(map[string]interface{}))
				}
			}

			collection.Files[0].Fields = append(collection.Files[0].Fields, field)
		}

		collections = append(collections, collection)
	}

	// Ensure the output directory exists
	err = os.MkdirAll(filepath.Dir(outputFile), os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating output directory %s: %v", filepath.Dir(outputFile), err)
	}

	err = writeCollections(collections, templateData, indexHTML, outputFile)
	if err != nil {
		return fmt.Errorf("error writing config: %v", err)
	}

	return nil
}

func formatPlaceholders(placeholders map[string]interface{}) string {
	var formatted []string
	for name, details := range placeholders {
		if detailMap, ok := details.(map[string]interface{}); ok {
			placeholderType := detailMap["type"]
			example := detailMap["example"]
			formatted = append(formatted, fmt.Sprintf("%s (type: %v, example: %v)", name, placeholderType, example))
		}
	}
	return "Placeholders: " + strings.Join(formatted, ", ")
}

func extractLanguage(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	parts := strings.Split(strings.TrimSuffix(base, ext), "_")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func getOrderedKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	// Sort keys to maintain order (adjust if necessary to match original order)
	sort.Strings(keys)
	return keys
}

// OrderedMap is used to preserve the order of keys in JSON
type KVPair struct {
	Key   string
	Value interface{}
}

type OrderedMap []KVPair

func (om *OrderedMap) UnmarshalJSON(data []byte) error {
	var tempMap map[string]interface{}
	if err := json.Unmarshal(data, &tempMap); err != nil {
		return err
	}

	for key, value := range tempMap {
		*om = append(*om, KVPair{Key: key, Value: value})
	}
	return nil
}

func (om OrderedMap) MarshalJSON() ([]byte, error) {
	tempMap := make(map[string]interface{})
	for _, kv := range om {
		tempMap[kv.Key] = kv.Value
	}
	return json.MarshalIndent(tempMap, "", "  ")
}

func (om OrderedMap) Get(key string) (interface{}, bool) {
	for _, kv := range om {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return nil, false
}

// YAMLToMapStringInterface unmarshals YAML into map[string]interface{}
func YAMLToMapStringInterface(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
