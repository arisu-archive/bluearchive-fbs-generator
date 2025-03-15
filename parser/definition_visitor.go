package parser

import (
	"strconv"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// DefinitionVisitor handles type definitions (table, struct, enum, union)
type DefinitionVisitor struct {
	Definitions  []Definition
	fieldVisitor *FieldVisitor
}

// NewDefinitionVisitor creates a new DefinitionVisitor
func NewDefinitionVisitor() *DefinitionVisitor {
	return &DefinitionVisitor{
		Definitions:  []Definition{},
		fieldVisitor: NewFieldVisitor(),
	}
}

// Visit implements NodeVisitor
func (v *DefinitionVisitor) Visit(node *sitter.Node, content []byte) error {
	switch node.Kind() {
	case "table_declaration":
		v.visitTable(node, content)
	case "struct_declaration":
		v.visitStruct(node, content)
	case "enum_declaration":
		v.visitEnum(node, content)
	case "union_declaration":
		v.visitUnion(node, content)
	}
	return nil
}

func (v *DefinitionVisitor) visitTable(node *sitter.Node, content []byte) {
	name := GetFieldText(node, "name", content)
	if name == "" {
		return
	}

	def := Definition{
		Name:   name,
		Type:   TypeTable,
		Fields: []Field{},
	}

	// Find and visit fields
	v.visitFields(node, &def, content)

	v.Definitions = append(v.Definitions, def)
}

func (v *DefinitionVisitor) visitStruct(node *sitter.Node, content []byte) {
	name := GetFieldText(node, "name", content)
	if name == "" {
		return
	}

	def := Definition{
		Name:   name,
		Type:   TypeStruct,
		Fields: []Field{},
	}

	// Find and visit fields
	v.visitFields(node, &def, content)

	v.Definitions = append(v.Definitions, def)
}

func (v *DefinitionVisitor) visitEnum(node *sitter.Node, content []byte) {
	name := GetFieldText(node, "name", content)
	if name == "" {
		return
	}

	def := Definition{
		Name:       name,
		Type:       TypeEnum,
		Fields:     []Field{},
		EnumValues: make(map[string]int),
	}

	// Find enum_values node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "enum_values" {
			v.visitEnumValues(child, &def, content)
			break
		}
	}

	v.Definitions = append(v.Definitions, def)
}

func (v *DefinitionVisitor) visitUnion(node *sitter.Node, content []byte) {
	name := GetFieldText(node, "name", content)
	if name == "" {
		return
	}

	def := Definition{
		Name:   name,
		Type:   TypeUnion,
		Fields: []Field{},
	}

	// Find union_values node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "union_values" {
			v.visitUnionValues(child, &def, content)
			break
		}
	}

	v.Definitions = append(v.Definitions, def)
}

func (v *DefinitionVisitor) visitFields(node *sitter.Node, def *Definition, content []byte) {
	// Find fields node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "fields" {
			// Process each field
			for j := range child.ChildCount() {
				fieldNode := child.Child(j)
				if fieldNode.Kind() == "field" {
					field := v.fieldVisitor.Visit(fieldNode, content)
					def.Fields = append(def.Fields, field)
				}
			}
			break
		}
	}
}

func (v *DefinitionVisitor) visitEnumValues(node *sitter.Node, def *Definition, content []byte) {
	value := 0

	// Process each enum_value
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "enum_value" {
			name := GetFieldText(child, "name", content)
			if name == "" {
				continue
			}

			// Check for explicit value
			for j := range child.ChildCount() {
				valChild := child.Child(j)
				if valChild.Kind() == "=" && j+1 < child.ChildCount() {
					intNode := child.Child(j + 1)
					if intNode.Kind() == "integer" {
						intStr := NodeText(intNode, content)
						if parsedVal, err := strconv.Atoi(intStr); err == nil {
							value = parsedVal
						}
					}
					break
				}
			}

			def.EnumValues[name] = value
			value++
		}
	}
}

func (v *DefinitionVisitor) visitUnionValues(node *sitter.Node, def *Definition, content []byte) {
	// Process each union_value
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "union_value" {
			name := GetFieldText(child, "name", content)
			if name == "" {
				continue
			}

			field := Field{
				Name:    name,
				Type:    name,
				IsUnion: true,
			}

			def.Fields = append(def.Fields, field)
		}
	}
}
