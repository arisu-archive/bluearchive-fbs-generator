package parser

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Visitor defines the interface for tree traversal
type Visitor interface {
	Visit(node *sitter.Node, content []byte) error
}

// NodeText extracts the text content of a node
func NodeText(node *sitter.Node, content []byte) string {
	return string(content[node.StartByte():node.EndByte()])
}

// GetNodeByFieldName safely retrieves a node by field name
func GetNodeByFieldName(node *sitter.Node, fieldName string) *sitter.Node {
	if node == nil {
		return nil
	}
	return node.ChildByFieldName(fieldName)
}

// GetFieldText retrieves text from a named field
func GetFieldText(node *sitter.Node, fieldName string, content []byte) string {
	fieldNode := GetNodeByFieldName(node, fieldName)
	if fieldNode == nil {
		return ""
	}
	return NodeText(fieldNode, content)
}

// isUpperFirstChar checks if the first character of a string is uppercase
func isUpperFirstChar(s string) bool {
	if len(s) > 0 {
		return s[0] >= 'A' && s[0] <= 'Z'
	}
	return false
}
