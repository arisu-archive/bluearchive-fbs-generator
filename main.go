package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arisu-archive/bluearchive-fbs-generator/generator"
	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func main() {
	var inputPath, outputDir, packageName string

	flag.StringVar(&inputPath, "i", "", "Input directory containing .fbs files (required)")
	flag.StringVar(&outputDir, "o", ".", "Output directory for generated Go files")
	flag.StringVar(&packageName, "p", "model", "Package name for generated Go files")
	flag.Parse()

	if inputPath == "" {
		fmt.Println("Error: Input file is required")
		flag.Usage()
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Glob all the files in the input directory
	files, err := filepath.Glob(filepath.Join(inputPath, "*.fbs"))
	if err != nil {
		fmt.Printf("Error globbing files: %v\n", err)
		os.Exit(1)
	}

	// Parse the schema
	ctx := context.Background()
	for _, file := range files {
		schema, err := parser.ParseFile(ctx, file)
		if err != nil {
			fmt.Printf("Error parsing schema: %v\n", err)
			os.Exit(1)
		}

		// Generate code
		if genErr := generator.Generate(schema, packageName, outputDir); genErr != nil {
			fmt.Printf("Error generating code: %v\n", genErr)
			os.Exit(1)
		}

		fmt.Printf("Generated code for %s\n", schema.FileName)
	}

	fmt.Println("Code generation completed successfully")
}
