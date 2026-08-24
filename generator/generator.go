package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

var (
	// ErrInvalidInput indicates that Render received an unusable schema or option.
	ErrInvalidInput = errors.New("generator: invalid input")
	// ErrUnsupportedSchema indicates that a schema uses a construct the generator
	// cannot emit correctly.
	ErrUnsupportedSchema = errors.New("generator: unsupported schema")
)

// Options configures generated source rendering.
type Options struct {
	// PackageName is the Go package declaration used in generated files.
	PackageName string
	// WithoutDecryption omits all table-key and field-conversion operations.
	WithoutDecryption bool
}

// GeneratedFile is one formatted source file produced by Render.
type GeneratedFile struct {
	// Name is a base filename safe to join to a caller-owned output directory.
	Name string
	// Content is complete, gofmt-compatible Go source owned by the caller.
	Content []byte
}

// Generate produces Go code from a FlatBuffers schema.
func Generate(s *parser.Schema, pkgName, outputDir string, withoutDecryption bool) error {
	files, err := Render(s, Options{
		PackageName:       pkgName,
		WithoutDecryption: withoutDecryption,
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", outputDir, err)
	}

	for _, file := range files {
		if err := writeFile(file, outputDir); err != nil {
			return err
		}
	}

	return nil
}

// Render produces formatted Go source without writing to the filesystem.
func Render(s *parser.Schema, options Options) ([]GeneratedFile, error) {
	if err := validateSchema(s, options); err != nil {
		return nil, err
	}

	file, filename, written := buildModelFile(s, options.PackageName, options.WithoutDecryption)
	if !written {
		return []GeneratedFile{}, nil
	}

	var output bytes.Buffer
	if err := file.Render(&output); err != nil {
		return nil, fmt.Errorf("render model %q: %w", filename, err)
	}

	return []GeneratedFile{{Name: filename, Content: output.Bytes()}}, nil
}

func validateSchema(s *parser.Schema, options Options) error {
	if s == nil {
		return fmt.Errorf("%w: schema is nil", ErrInvalidInput)
	}
	if options.PackageName == "_" || !token.IsIdentifier(options.PackageName) {
		return fmt.Errorf("%w: invalid package name %q", ErrInvalidInput, options.PackageName)
	}
	if strings.TrimSpace(s.FileName) == "" || strings.ContainsAny(s.FileName, `/\`) {
		return fmt.Errorf("%w: invalid schema file name %q", ErrInvalidInput, s.FileName)
	}

	for _, definition := range s.Definitions {
		switch definition.Type {
		case parser.TypeStruct:
			return fmt.Errorf("%w: struct %q", ErrUnsupportedSchema, definition.Name)
		case parser.TypeUnion:
			return fmt.Errorf("%w: union %q", ErrUnsupportedSchema, definition.Name)
		}

		for _, field := range definition.Fields {
			if field.IsStruct || field.IsUnion {
				return fmt.Errorf(
					"%w: field %q.%s references %s %q",
					ErrUnsupportedSchema,
					definition.Name,
					field.Name,
					unsupportedFieldKind(field),
					field.Type,
				)
			}
			if !field.IsPrimitive() && !field.IsTable && !field.IsEnum {
				return fmt.Errorf(
					"%w: field %q.%s has unresolved type %q",
					ErrUnsupportedSchema,
					definition.Name,
					field.Name,
					field.Type,
				)
			}
		}
	}
	return nil
}

func unsupportedFieldKind(field parser.Field) string {
	if field.IsStruct {
		return "struct"
	}
	return "union"
}

func writeFile(file GeneratedFile, outputDir string) error {
	outputPath := filepath.Join(outputDir, file.Name)
	if err := os.WriteFile(outputPath, file.Content, 0o644); err != nil {
		return fmt.Errorf("write generated file %q: %w", outputPath, err)
	}
	return nil
}

// decodeValue generates a qualified call that invokes the Decode function in the bluearchive-fbs-utils package.
func decodeValue(value jen.Code) *jen.Statement {
	return jen.Qual("github.com/arisu-archive/bluearchive-fbs-utils", "Decode").Call(
		value,
		jen.Id("t").Dot("FlatBuffer").Dot("TableKey"),
	)
}
