package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flatbuffers "github.com/arisu-archive/tree-sitter-flatbuffers/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseFile parses a FlatBuffers schema file and returns a Schema
func ParseFile(ctx context.Context, path string) (*Schema, error) {
	fileName := strings.TrimSuffix(filepath.Base(path), ".fbs")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	parser := sitter.NewParser()
	if setErr := parser.SetLanguage(sitter.NewLanguage(flatbuffers.Language())); setErr != nil {
		return nil, fmt.Errorf("failed to set language: %w", setErr)
	}

	tree := parser.ParseCtx(ctx, content, nil)
	rootNode := tree.RootNode()

	// Create a visitor to build the schema
	visitor := NewSchemaVisitor()
	// Process each top-level node
	if err := visitor.Visit(rootNode, fileName, content); err != nil {
		return nil, fmt.Errorf("failed to visit root node: %w", err)
	}

	return visitor.GetSchema(), nil
}
