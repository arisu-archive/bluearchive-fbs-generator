package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
	"github.com/dave/jennifer/jen"
)

// Generate produces Go code from a FlatBuffers schema
func Generate(s *parser.Schema, pkgName, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate model code
	if err := generateModel(s, pkgName, outputDir); err != nil {
		return fmt.Errorf("model generation failed: %w", err)
	}

	return nil
}

// writeFile saves a jennifer file to disk
func writeFile(f *jen.File, outputDir, filename string) error {
	outPath := filepath.Join(outputDir, filename)
	return f.Save(outPath)
}
