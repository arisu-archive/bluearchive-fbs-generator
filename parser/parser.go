package parser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flatbuffers "github.com/arisu-archive/tree-sitter-flatbuffers/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

var (
	// ErrInvalidSchema indicates that a FlatBuffers schema could not be parsed.
	ErrInvalidSchema = errors.New("parser: invalid FlatBuffers schema")
	// ErrIncludeCycle indicates that schema includes form a cycle.
	ErrIncludeCycle = errors.New("parser: include cycle")
)

// ParseFile parses a FlatBuffers schema and its local includes. It returns
// ErrInvalidSchema for malformed syntax and ErrIncludeCycle for include cycles.
func ParseFile(ctx context.Context, path string) (*Schema, error) {
	return parseFile(ctx, path, make(map[string]struct{}))
}

func parseFile(ctx context.Context, path string, visiting map[string]struct{}) (*Schema, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	canonicalPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve schema path %q: %w", path, err)
	}
	canonicalPath = filepath.Clean(canonicalPath)
	if _, exists := visiting[canonicalPath]; exists {
		return nil, fmt.Errorf("%w at %q", ErrIncludeCycle, canonicalPath)
	}
	visiting[canonicalPath] = struct{}{}
	defer delete(visiting, canonicalPath)

	fileName := strings.TrimSuffix(filepath.Base(canonicalPath), ".fbs")
	content, err := os.ReadFile(canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %q: %w", canonicalPath, err)
	}

	syntaxParser := sitter.NewParser()
	defer syntaxParser.Close()
	if err := syntaxParser.SetLanguage(sitter.NewLanguage(flatbuffers.Language())); err != nil {
		return nil, fmt.Errorf("set FlatBuffers language: %w", err)
	}

	tree := syntaxParser.ParseCtx(ctx, content, nil)
	if tree == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: parser returned no syntax tree for %q", ErrInvalidSchema, canonicalPath)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode.HasError() {
		return nil, fmt.Errorf("%w: syntax error in %q", ErrInvalidSchema, canonicalPath)
	}

	visitor := NewSchemaVisitor()
	if err := visitor.Visit(rootNode, fileName, content); err != nil {
		return nil, fmt.Errorf("visit schema %q: %w", canonicalPath, err)
	}

	includedTypes, err := visitor.resolveIncludes(ctx, canonicalPath, visiting)
	if err != nil {
		return nil, fmt.Errorf("resolve includes for %q: %w", canonicalPath, err)
	}

	visitor.processFieldTypes(includedTypes)

	return visitor.GetSchema(), nil
}
