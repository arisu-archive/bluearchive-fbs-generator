package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/arisu-archive/bluearchive-fbs-generator/generator"
	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("fbsgen", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		inputPath         string
		outputDir         string
		packageName       string
		withoutDecryption bool
	)
	flags.StringVar(&inputPath, "i", "", "Input directory containing .fbs files (required)")
	flags.StringVar(&inputPath, "input", "", "Input directory containing .fbs files (required)")
	flags.StringVar(&outputDir, "o", ".", "Output directory for generated Go files")
	flags.StringVar(&outputDir, "output", ".", "Output directory for generated Go files")
	flags.StringVar(&packageName, "p", "model", "Package name for generated Go files")
	flags.StringVar(&packageName, "package", "model", "Package name for generated Go files")
	flags.BoolVar(
		&withoutDecryption,
		"without-decryption",
		false,
		"Skip field encryption and decryption conversions",
	)

	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}
	if inputPath == "" {
		return errors.New("input directory is required")
	}

	files, err := filepath.Glob(filepath.Join(inputPath, "*.fbs"))
	if err != nil {
		return fmt.Errorf("find schemas in %q: %w", inputPath, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no .fbs files found in %q", inputPath)
	}

	for _, file := range files {
		schema, err := parser.ParseFile(ctx, file)
		if err != nil {
			return fmt.Errorf("parse schema %q: %w", file, err)
		}
		if err := generator.Generate(schema, packageName, outputDir, withoutDecryption); err != nil {
			return fmt.Errorf("generate schema %q: %w", file, err)
		}
		if _, err := fmt.Fprintf(stdout, "Generated code for %s\n", schema.FileName); err != nil {
			return fmt.Errorf("write generation result: %w", err)
		}
	}

	if _, err := fmt.Fprintln(stdout, "Code generation completed successfully"); err != nil {
		return fmt.Errorf("write completion result: %w", err)
	}
	return nil
}
