package parser

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// SchemaVisitor combines multiple visitors
type SchemaVisitor struct {
	infoVisitor       *InfoVisitor
	definitionVisitor *DefinitionVisitor
	schema            *Schema
}

// NewSchemaVisitor creates a new SchemaVisitor
func NewSchemaVisitor() *SchemaVisitor {
	return &SchemaVisitor{
		infoVisitor:       NewInfoVisitor(),
		definitionVisitor: NewDefinitionVisitor(),
		schema:            &Schema{},
	}
}

// Visit dispatches the node to the appropriate specialized visitor
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
				v.infoVisitor.Visit(declaration, content)

				// Process definition-related declarations
				v.definitionVisitor.Visit(declaration, content)
			}
		}
	}

	// Build the final schema
	v.buildSchema(fileName)

	return nil
}

// GetSchema returns the built schema
func (v *SchemaVisitor) GetSchema() *Schema {
	return v.schema
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
	v.processFieldTypes()
}

// processFieldTypes sets flags on fields based on definition types
func (v *SchemaVisitor) processFieldTypes() {
	// Build a type map for lookups
	typeMap := make(map[string]SchemaType)
	for _, def := range v.schema.Definitions {
		typeMap[def.Name] = def.Type
	}

	// Process fields in all definitions
	for i := range v.schema.Definitions {
		def := &v.schema.Definitions[i]
		for j := range def.Fields {
			field := &def.Fields[j]

			// Skip fields that already have type flags set
			if field.IsString || field.IsVector || field.IsUnion {
				continue
			}

			// Check if type exists in our definitions
			if schemaType, exists := typeMap[field.Type]; exists {
				switch schemaType {
				case TypeStruct:
					field.IsStruct = true
				case TypeEnum:
					field.IsEnum = true
				case TypeUnion:
					field.IsUnion = true
				}
			}
		}
	}
}
