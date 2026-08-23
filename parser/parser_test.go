package parser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileRejectsMalformedSchema(t *testing.T) {
	t.Parallel()

	path := writeSchema(t, "broken.fbs", "table Broken { value:int; ")
	_, err := ParseFile(context.Background(), path)
	if !errors.Is(err, ErrInvalidSchema) {
		t.Errorf("ParseFile(malformed schema) error = %v, want errors.Is(ErrInvalidSchema)", err)
	}
}

func TestParseFileHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	path := writeSchema(t, "valid.fbs", "table Valid { value:int; }")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ParseFile(ctx, path)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ParseFile(canceled context) error = %v, want errors.Is(context.Canceled)", err)
	}
}

func TestParseFileRejectsIncludeCycle(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	aPath := filepath.Join(directory, "a.fbs")
	bPath := filepath.Join(directory, "b.fbs")
	if err := os.WriteFile(aPath, []byte("include \"b.fbs\"; table A {}"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", aPath, err)
	}
	if err := os.WriteFile(bPath, []byte("include \"a.fbs\"; table B {}"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", bPath, err)
	}

	_, err := ParseFile(context.Background(), aPath)
	if !errors.Is(err, ErrIncludeCycle) {
		t.Errorf("ParseFile(include cycle) error = %v, want errors.Is(ErrIncludeCycle)", err)
	}
}

func writeSchema(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}
