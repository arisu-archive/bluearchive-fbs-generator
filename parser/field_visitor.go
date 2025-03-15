package parser

import (
	"strconv"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// FieldVisitor handles field declarations
type FieldVisitor struct{}

// NewFieldVisitor creates a new FieldVisitor
func NewFieldVisitor() *FieldVisitor {
	return &FieldVisitor{}
}

// Visit processes a field declaration and returns a Field
func (v *FieldVisitor) Visit(node *sitter.Node, content []byte) Field {
	field := Field{}

	// Get field name
	field.Name = GetFieldText(node, "name", content)

	// Process field type
	typeNode := GetNodeByFieldName(node, "type")
	if typeNode != nil {
		v.processFieldType(typeNode, &field, content)
	}

	// Process metadata if present
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "field_metadata" {
			v.processFieldMetadata(child, &field, content)
			break
		}
	}

	return field
}

func (v *FieldVisitor) processFieldType(node *sitter.Node, field *Field, content []byte) {
	switch node.Kind() {
	case "type":
		// For type nodes, look at the first child to determine the actual type
		if node.ChildCount() > 0 {
			childNode := node.Child(0)
			v.processFieldType(childNode, field, content)
		} else {
			field.Type = NodeText(node, content)
		}
	case "vector_type":
		field.IsVector = true
		// Get the contained type (should be inside the [] brackets)
		for i := range node.ChildCount() {
			child := node.Child(i)
			// Skip the [] bracket nodes
			if child.Kind() != "[" && child.Kind() != "]" {
				// Process the type inside the vector
				v.processFieldType(child, field, content)
				break
			}
		}
	case "string_type":
		field.Type = "string"
		field.IsString = true
	case "scalar_type":
		field.Type = NodeText(node, content)
	case "identifier":
		// User-defined type
		field.Type = NodeText(node, content)
		// Type flags will be set by composite visitor
	default:
		field.Type = NodeText(node, content)
	}
}

func (v *FieldVisitor) processFieldMetadata(node *sitter.Node, field *Field, content []byte) {
	// Find metadata_values node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "metadata_values" {
			// Process each metadata_value
			for j := range child.ChildCount() {
				metaNode := child.Child(j)
				if metaNode.Kind() == "metadata_value" {
					v.processMetadataValue(metaNode, field, content)
				}
			}
			break
		}
	}
}

func (v *FieldVisitor) processMetadataValue(node *sitter.Node, field *Field, content []byte) {
	key := GetFieldText(node, "key", content)

	// Process known metadata keys
	switch key {
	case "id":
		valueNode := GetNodeByFieldName(node, "value")
		if valueNode != nil && (valueNode.Kind() == "number" || valueNode.Kind() == "integer") {
			// This is simplified - in a real impl you'd parse the int properly
			idStr := NodeText(valueNode, content)
			field.ID, _ = strconv.Atoi(idStr)
		}
	case "required":
		// Handle required flag
	case "deprecated":
		// Handle deprecated flag
	}
}
