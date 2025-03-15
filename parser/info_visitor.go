package parser

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// InfoVisitor handles schema-level information (namespace, includes, root_type)
type InfoVisitor struct {
	Namespace string
	Includes  []string
	RootTable string
	FileExt   string
	FileID    string
}

// NewInfoVisitor creates a new InfoVisitor
func NewInfoVisitor() *InfoVisitor {
	return &InfoVisitor{
		Includes: []string{},
	}
}

// Visit implements NodeVisitor
func (v *InfoVisitor) Visit(node *sitter.Node, content []byte) error {
	switch node.Kind() {
	case "namespace_declaration":
		v.visitNamespace(node, content)
	case "include_declaration":
		v.visitInclude(node, content)
	case "root_type_declaration":
		v.visitRootType(node, content)
	case "file_extension_declaration":
		v.visitFileExtension(node, content)
	case "file_identifier_declaration":
		v.visitFileIdentifier(node, content)
	}
	return nil
}

func (v *InfoVisitor) visitNamespace(node *sitter.Node, content []byte) {
	// Find namespace_identifier node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "namespace_identifier" {
			v.Namespace = NodeText(child, content)
			return
		}
	}
}

func (v *InfoVisitor) visitInclude(node *sitter.Node, content []byte) {
	// Find string node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "string" {
			includeStr := NodeText(child, content)
			// Remove quotes from string
			includeStr = strings.Trim(includeStr, "\"'")
			v.Includes = append(v.Includes, includeStr)
			return
		}
	}
}

func (v *InfoVisitor) visitRootType(node *sitter.Node, content []byte) {
	// Find identifier node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "identifier" {
			v.RootTable = NodeText(child, content)
			return
		}
	}
}

func (v *InfoVisitor) visitFileExtension(node *sitter.Node, content []byte) {
	// Find string node
	for i := range node.ChildCount() {
		child := node.Child(i)
		if child.Kind() == "string" {
			extStr := NodeText(child, content)
			// Remove quotes from string
			v.FileExt = strings.Trim(extStr, "\"'")
			return
		}
	}
}

func (v *InfoVisitor) visitFileIdentifier(node *sitter.Node, content []byte) {
	// Find string node
	for i := range node.ChildCount() {
		child := node.Child(uint(i))
		if child.Kind() == "string" {
			idStr := NodeText(child, content)
			// Remove quotes from string
			v.FileID = strings.Trim(idStr, "\"'")
			return
		}
	}
}
