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

// main.go
package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kyodo-tech/decapta/model"
	"github.com/spf13/cobra"
)

//go:embed template/config.yml
var embeddedTemplateConfigSample []byte

//go:embed template/index.html
var indexHTML []byte

func main() {
	var dataType string
	var dataDir string
	var outputFile string
	var templateFile string
	var contentDir string
	var slugFields string
	var ignoreFiles string

	var rootCmd = &cobra.Command{
		Use:   "decapta",
		Short: "decapta is a tool for managing data with Decap CMS",
	}

	var preProcessCmd = &cobra.Command{
		Use:   "pre-process",
		Short: "Pre-process data for Decap CMS",
		Run: func(cmd *cobra.Command, args []string) {
			ignoredFiles := strings.Split(ignoreFiles, ",")

			slugFieldList := []string{}
			if slugFields != "" {
				slugFieldList = strings.Split(slugFields, ",")
			}

			switch dataType {
			case "arb":
				err := model.ARBPreProcess(dataDir, contentDir)
				if err != nil {
					log.Fatalf("ARB Pre-Process Error: %v", err)
				}
			case "csv":
				err := model.CSVPreProcess(dataDir, contentDir, slugFieldList, ignoredFiles)
				if err != nil {
					log.Fatalf("CSV Pre-Process Error: %v", err)
				}
			default:
				fmt.Println("Unsupported data type:", dataType)
			}
		},
	}

	var postProcessCmd = &cobra.Command{
		Use:   "post-process",
		Short: "Post-process data from Decap CMS",
		Run: func(cmd *cobra.Command, args []string) {
			switch dataType {
			case "arb":
				err := model.ARBPostProcess(contentDir, dataDir)
				if err != nil {
					log.Fatalf("ARB Post-Process Error: %v", err)
				}
			case "csv":
				err := model.CSVPostProcess(contentDir, dataDir)
				if err != nil {
					log.Fatalf("CSV Post-Process Error: %v", err)
				}
			default:
				fmt.Println("Unsupported data type:", dataType)
			}
		},
	}

	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Generate config.yml for Decap CMS",
		Run: func(cmd *cobra.Command, args []string) {
			ignoredFiles := strings.Split(ignoreFiles, ",")

			var templateData []byte
			if templateFile == "" {
				templateData = embeddedTemplateConfigSample
			} else {
				// Load the config template
				templateContent, err := os.ReadFile(templateFile)
				if err != nil {
					log.Fatalf("Error reading template file: %v", err)
				}
				templateData = templateContent
			}

			switch dataType {
			case "arb":
				err := model.ARBGenerateConfig(dataDir, outputFile, templateData, indexHTML, contentDir)
				if err != nil {
					log.Fatalf("ARB Config Generation Error: %v", err)
				}
			case "csv":
				err := model.CSVGenerateConfig(dataDir, outputFile, templateData, indexHTML, contentDir, ignoredFiles)
				if err != nil {
					log.Fatalf("CSV Config Generation Error: %v", err)
				}
			default:
				fmt.Println("Unsupported data type:", dataType)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&dataType, "type", "t", "", "Data type (arb or csv)")
	rootCmd.MarkPersistentFlagRequired("type")

	preProcessCmd.Flags().StringVarP(&dataDir, "in", "i", "", "Directory containing data files ARB,CSV,etc.")
	preProcessCmd.Flags().StringVar(&contentDir, "content-dir", "content", "Content directory for CMS")
	preProcessCmd.Flags().StringVar(&slugFields, "slug", "", "Comma-separated list of fields to use for identifier_field (e.g., id,name,status)")
	preProcessCmd.Flags().StringVar(&ignoreFiles, "ignore-files", "", "Comma-separated list of filenames to ignore (e.g., interactions.csv,metadata.csv)")

	postProcessCmd.Flags().StringVar(&contentDir, "content-dir", "content", "Content directory for CMS")
	postProcessCmd.Flags().StringVarP(&dataDir, "out", "o", "", "Output directory to write ARB,CSV,etc. files")

	configCmd.Flags().StringVarP(&dataDir, "in", "i", "", "Directory containing data files ARB,CSV,etc.")
	configCmd.Flags().StringVarP(&outputFile, "output-file", "o", "admin/config.yml", "Output file for config")
	configCmd.Flags().StringVar(&templateFile, "template-file", "", "Template yml config file")
	configCmd.Flags().StringVar(&contentDir, "content-dir", "content", "Content directory for CMS")
	configCmd.Flags().StringVar(&ignoreFiles, "ignore-files", "", "Comma-separated list of filenames to ignore (e.g., interactions.csv,metadata.csv)")

	rootCmd.AddCommand(preProcessCmd)
	rootCmd.AddCommand(postProcessCmd)
	rootCmd.AddCommand(configCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
