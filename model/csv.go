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
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

func addPrefixIfReserved(fieldName string) string {
	if reservedFields()[fieldName] {
		return decaptaPrefix + fieldName
	}
	return fieldName
}

// CSVPreProcess reads CSV files and creates a file per CSV row for Decap CMS.
func CSVPreProcess(csvDir string, contentDir string, slugFields, ignoredFiles []string) error {
	files, err := os.ReadDir(csvDir)
	if err != nil {
		return fmt.Errorf("error reading CSV directory: %v", err)
	}

	for _, file := range files {
		if contains(ignoredFiles, file.Name()) {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		csvFilePath := filepath.Join(csvDir, file.Name())
		csvFile, err := os.Open(csvFilePath)
		if err != nil {
			return fmt.Errorf("error opening CSV file %s: %v", csvFilePath, err)
		}
		defer csvFile.Close()

		reader := csv.NewReader(csvFile)
		records, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("error reading CSV file %s: %v", csvFilePath, err)
		}

		if len(records) < 1 {
			continue // Empty CSV
		}

		headers := records[0]

		// Create content directory for CSV
		csvContentDir := filepath.Join(contentDir, strings.TrimSuffix(file.Name(), ".csv"))
		err = os.MkdirAll(csvContentDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error creating content directory %s: %v", csvContentDir, err)
		}

		// Store column order at the project level (one directory higher)
		columnOrderFilePath := filepath.Join(contentDir, fmt.Sprintf(".%s.yaml", strings.TrimSuffix(file.Name(), ".csv")))
		adjustedHeaders := make([]string, len(headers))
		for i, header := range headers {
			adjustedHeaders[i] = addPrefixIfReserved(header)
		}
		err = writeColumnOrder(adjustedHeaders, columnOrderFilePath)
		if err != nil {
			return fmt.Errorf("error writing column order to file %s: %v", columnOrderFilePath, err)
		}

		// Process each row and create a YAML file
		for i, record := range records[1:] {
			data := make(map[string]interface{})
			for j, value := range record {
				if j < len(headers) {
					prefixedHeader := addPrefixIfReserved(headers[j])
					data[prefixedHeader] = value
				}
			}

			// Generate idField by concatenating specified fields
			idField := generateIdentifierField(data, slugFields)
			data[decaptaIDField] = idField

			// Write YAML file (1.yaml, 2.yaml, etc.)
			filename := filepath.Join(csvContentDir, fmt.Sprintf("%d.yaml", i+1))
			yamlData, err := yaml.Marshal(data)
			if err != nil {
				return fmt.Errorf("error marshaling YAML for row %d: %v", i+1, err)
			}

			err = os.WriteFile(filename, yamlData, 0644)
			if err != nil {
				return fmt.Errorf("error writing YAML file %s: %v", filename, err)
			}
		}
	}

	return nil
}

func generateIdentifierField(data map[string]interface{}, fields []string) string {
	var slugParts []string
	for _, field := range fields {
		if value, exists := data[field]; exists {
			slugParts = append(slugParts, fmt.Sprintf("%v", value))
		}
	}
	return strings.Join(slugParts, "-")
}

func writeColumnOrder(headers []string, filepath string) error {
	columnOrder := map[string][]string{
		"columns": headers,
	}
	yamlData, err := yaml.Marshal(columnOrder)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, yamlData, 0644)
}

func removePrefixIfReserved(fieldName string) string {
	if strings.HasPrefix(fieldName, decaptaPrefix) && reservedFields()[strings.TrimPrefix(fieldName, decaptaPrefix)] {
		return strings.TrimPrefix(fieldName, decaptaPrefix)
	}
	return fieldName
}

// CSVPostProcess reads the content files and recreates the CSV files.
func CSVPostProcess(contentDir string, csvDir string) error {
	csvContentDirs, err := os.ReadDir(contentDir)
	if err != nil {
		return fmt.Errorf("error reading content directory: %v", err)
	}

	// Ensure the output directory exists
	err = os.MkdirAll(csvDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating CSV directory %s: %v", csvDir, err)
	}

	for _, dir := range csvContentDirs {
		if !dir.IsDir() {
			continue
		}

		csvName := dir.Name()
		csvContentDir := filepath.Join(contentDir, csvName)

		files, err := os.ReadDir(csvContentDir)
		if err != nil {
			return fmt.Errorf("error reading CSV content directory %s: %v", csvContentDir, err)
		}

		var records []map[string]interface{}
		var headers []string

		// Read the column order from the project-level metadata file
		columnOrderFilePath := filepath.Join(contentDir, fmt.Sprintf(".%s.yaml", csvName))
		headers, err = readColumnOrder(columnOrderFilePath)
		if err != nil {
			return fmt.Errorf("error reading column order from file %s: %v", columnOrderFilePath, err)
		}

		// Adjust headers to match the prefixed keys in content data
		adjustedHeaders := make([]string, len(headers))
		for i, header := range headers {
			adjustedHeaders[i] = addPrefixIfReserved(header) // Use prefixed name for reading
		}

		// Read YAML files, sort them by their numeric filename (1.yaml, 2.yaml, etc.)
		sort.Slice(files, func(i, j int) bool {
			num1 := extractFileNumber(files[i].Name())
			num2 := extractFileNumber(files[j].Name())
			return num1 < num2
		})

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".yaml") || file.Name() == fmt.Sprintf(".%s.yaml", csvName) {
				continue
			}

			filePath := filepath.Join(csvContentDir, file.Name())
			yamlContent, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("error reading YAML file %s: %v", filePath, err)
			}

			var data map[string]interface{}
			err = yaml.Unmarshal(yamlContent, &data)
			if err != nil {
				return fmt.Errorf("error unmarshaling YAML file %s: %v", filePath, err)
			}

			records = append(records, data)
		}

		// Write CSV file
		csvFilePath := filepath.Join(csvDir, fmt.Sprintf("%s.csv", csvName))
		csvFile, err := os.Create(csvFilePath)
		if err != nil {
			return fmt.Errorf("error creating CSV file %s: %v", csvFilePath, err)
		}
		defer csvFile.Close()

		writer := csv.NewWriter(csvFile)

		// Write headers with prefixes removed
		originalHeaders := make([]string, len(headers))
		for i, header := range headers {
			originalHeaders[i] = removePrefixIfReserved(header)
		}
		writer.Write(originalHeaders)

		// Write records
		for _, record := range records {
			var row []string
			for _, header := range headers {
				prefixedHeader := addPrefixIfReserved(header) // Use prefixed name to fetch data from record
				value := ""
				if val, ok := record[prefixedHeader]; ok {
					value = fmt.Sprintf("%v", val)
				}
				row = append(row, value)
			}
			writer.Write(row)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("error writing CSV file %s: %v", csvFilePath, err)
		}
	}

	return nil
}

func contains(slice []string, element string) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

func readColumnOrder(filepath string) ([]string, error) {
	var columnOrder struct {
		Columns []string `yaml:"columns"`
	}
	yamlContent, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlContent, &columnOrder)
	if err != nil {
		return nil, err
	}
	return columnOrder.Columns, nil
}

// extractFileNumber extracts the numeric ID from file name (e.g., "1.yaml" -> 1)
func extractFileNumber(filename string) int {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	num, err := strconv.Atoi(name)
	if err != nil {
		return -1 // Handle error, -1 for invalid numbers
	}
	return num
}

// CSVGenerateConfig generates the config.yml for CSV files.
func CSVGenerateConfig(csvDir string, outputFile string, templateData, indexHTML []byte, contentDir string, ignoredFiles []string) error {
	files, err := os.ReadDir(csvDir)
	if err != nil {
		return fmt.Errorf("error reading CSV directory: %v", err)
	}

	var collections []Collection

	for _, file := range files {
		if contains(ignoredFiles, file.Name()) {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		csvFilePath := filepath.Join(csvDir, file.Name())
		csvFile, err := os.Open(csvFilePath)
		if err != nil {
			return fmt.Errorf("error opening CSV file %s: %v", csvFilePath, err)
		}
		defer csvFile.Close()

		reader := csv.NewReader(csvFile)
		records, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("error reading CSV file %s: %v", csvFilePath, err)
		}

		if len(records) < 1 {
			continue // Empty CSV
		}

		headers := records[0]
		csvName := strings.TrimSuffix(file.Name(), ".csv")
		csvContentDir := filepath.Join(contentDir, csvName)

		// Generate fields based on headers
		var fields []Field

		// add the decapta_id field
		fields = append(fields, Field{
			Label:    "Decapta ID",
			Name:     decaptaIDField,
			Widget:   "string",
			Required: true,
		})

		for colIndex, header := range headers {
			fieldType := detectFieldType(records[1:], colIndex)

			field := Field{
				Label:    header,
				Name:     addPrefixIfReserved(header),
				Widget:   fieldType,
				Required: false,
			}

			// If the field is detected as markdown, specify modes
			if fieldType == "markdown" {
				field.Modes = []string{"raw"}
			}

			fields = append(fields, field)
		}

		collection := Collection{
			Name:      fmt.Sprintf("csv_%s", csvName),
			Label:     fmt.Sprintf("CSV Data (%s)", csvName),
			Slug:      "{{slug}}",
			Folder:    csvContentDir,
			Create:    true,
			Extension: "yaml",
			Format:    "yaml",
			Editor: Editor{
				Preview: false,
			},
			IdentifierField: decaptaIDField,
			Fields:          fields,
		}

		collections = append(collections, collection)
	}

	err = writeCollections(collections, templateData, indexHTML, outputFile)
	if err != nil {
		return fmt.Errorf("error writing config: %v", err)
	}

	return nil
}

// detectFieldType analyzes a column and returns the appropriate FieldType based on sample data.
func detectFieldType(records [][]string, colIndex int) string {
	var isMultilineText bool
	isFloat, isBoolean, isDate := true, true, true

	for _, record := range records {
		if colIndex >= len(record) {
			continue
		}
		value := record[colIndex]

		// Check for multiline (Markdown) by detecting line breaks
		if strings.Contains(value, "\n") {
			isMultilineText = true
		}

		// Try parsing as float
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			isFloat = false
		}

		// Try parsing as boolean
		lowerValue := strings.ToLower(value)
		if lowerValue != "true" && lowerValue != "false" && lowerValue != "yes" && lowerValue != "no" {
			isBoolean = false
		}

		// Try parsing as date
		if _, err := parseDate(value); err != nil {
			isDate = false
		}
	}

	// Determine widget type based on flags
	switch {
	case isMultilineText:
		return "markdown"
	case isBoolean:
		return "boolean"
	case isDate:
		return "datetime"
	case isFloat:
		return "number"
	default:
		return "string"
	}
}

// parseDate attempts to parse a string into a date with common formats
func parseDate(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339, "2006-01-02", "02/01/2006", "01/02/2006", "2006/01/02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a date")
}
