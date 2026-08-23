package parser

import (
	"context"
	"fmt"
	"path/filepath"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// SchemaVisitor combines schema metadata and definition visitors.
type SchemaVisitor struct {
	infoVisitor       *InfoVisitor
	definitionVisitor *DefinitionVisitor
	schema            *Schema
}

// NewSchemaVisitor creates a SchemaVisitor.
func NewSchemaVisitor() *SchemaVisitor {
	return &SchemaVisitor{
		infoVisitor:       NewInfoVisitor(),
		definitionVisitor: NewDefinitionVisitor(),
		schema:            &Schema{},
	}
}

// Visit dispatches a syntax node to the specialized visitors.
func (v *SchemaVisitor) Visit(node *sitter.Node, fileName string, content []byte) error {
	// Only process source_file at the top level
	if node.Kind() != "source_file" {
		return nil
	}

	// Visit all children of source_file
	for i := range node.ChildCount() {
		child := node.Child(i)
		// Handle declaration nodes by processing their children
		if child.Kind() == "declaration" && child.ChildCount() > 0 {
			// Get the actual declaration (first child of declaration node)
			declaration := child.Child(0)
			if declaration != nil {
				// Process info-related declarations
				if err := v.infoVisitor.Visit(declaration, content); err != nil {
					return fmt.Errorf("visit schema information: %w", err)
				}

				// Process definition-related declarations
				if err := v.definitionVisitor.Visit(declaration, content); err != nil {
					return fmt.Errorf("visit schema definition: %w", err)
				}
			}
		}
	}

	// Build the final schema
	v.buildSchema(fileName)

	return nil
}

// GetSchema returns the built schema.
func (v *SchemaVisitor) GetSchema() *Schema {
	return v.schema
}

// ResolveIncludes parses included schemas and returns their type definitions.
func (v *SchemaVisitor) ResolveIncludes(ctx context.Context, basePath string) (map[string]SchemaType, error) {
	canonicalPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve schema path %q: %w", basePath, err)
	}
	canonicalPath = filepath.Clean(canonicalPath)
	visiting := map[string]struct{}{canonicalPath: {}}
	return v.resolveIncludes(ctx, canonicalPath, visiting)
}

func (v *SchemaVisitor) resolveIncludes(
	ctx context.Context,
	basePath string,
	visiting map[string]struct{},
) (map[string]SchemaType, error) {
	typeMap := make(map[string]SchemaType)

	// Process each include
	for _, include := range v.schema.Includes {
		includePath := filepath.Join(filepath.Dir(basePath), include)

		// Parse the included schema file
		includeSchema, err := parseFile(ctx, includePath, visiting)
		if err != nil {
			return nil, fmt.Errorf("failed to parse included file %s: %w", include, err)
		}

		// Add all definitions from the included schema to our type map
		for _, def := range includeSchema.Definitions {
			typeMap[def.Name] = def.Type
			// If namespace is present, also add with fully qualified name
			if includeSchema.Namespace != "" {
				typeMap[includeSchema.Namespace+"."+def.Name] = def.Type
			}
		}
	}

	return typeMap, nil
}

func (v *SchemaVisitor) buildSchema(fileName string) {
	// Copy info
	v.schema.FileName = fileName
	v.schema.Namespace = v.infoVisitor.Namespace
	v.schema.Includes = v.infoVisitor.Includes
	v.schema.RootTable = v.infoVisitor.RootTable
	// Copy and post-process definitions
	v.schema.Definitions = v.definitionVisitor.Definitions

	// Post-process field types to set IsStruct, IsEnum, etc.
	typeMap := make(map[string]SchemaType)
	for _, def := range v.schema.Definitions {
		typeMap[def.Name] = def.Type
	}
	v.processFieldTypes(typeMap)
}

// processFieldTypes processes field types using definitions from included schemas
func (v *SchemaVisitor) processFieldTypes(typeMap map[string]SchemaType) {
	// Process fields in all definitions
	for i := range v.schema.Definitions {
		def := &v.schema.Definitions[i]
		for j := range def.Fields {
			field := &def.Fields[j]

			// Skip fields that already have type flags set correctly
			if field.IsStruct || field.IsEnum || field.IsUnion || field.IsTable {
				continue
			}

			// Check if the type exists in included schemas
			if schemaType, exists := typeMap[field.Type]; exists {
				switch schemaType {
				case TypeStruct:
					field.IsStruct = true
				case TypeTable:
					field.IsTable = true
				case TypeEnum:
					field.IsEnum = true
				case TypeUnion:
					field.IsUnion = true
				}
			}
		}
	}
}
