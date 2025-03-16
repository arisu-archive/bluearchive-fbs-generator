package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dave/jennifer/jen"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

// Generate produces Go code from a FlatBuffers schema.
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

// writeFile saves a jennifer file to disk.
func writeFile(f *jen.File, outputDir, filename string) error {
	outPath := filepath.Join(outputDir, filename)
	return f.Save(outPath)
}

// fieldConverter generates a qualified call that invokes the Convert function in the bluearchive-fbs-utils package.
func fieldConverter(field jen.Code) *jen.Statement {
	return jen.Qual("github.com/arisu-archive/bluearchive-fbs-utils", "Convert").Call(
		field,
		jen.Id("t").Dot("FlatBuffer").Dot("TableKey"),
	)
}
