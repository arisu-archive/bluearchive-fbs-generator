package generator

import (
	"context"
	"encoding/json"
	"errors"
	goast "go/ast"
	goParser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

type renderGolden struct {
	FileName           string   `json:"file_name"`
	RequiredFragments  []string `json:"required_fragments"`
	ForbiddenFragments []string `json:"forbidden_fragments"`
}

func TestRenderRejectsUnsupportedSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition parser.Definition
		options    Options
	}{
		{
			name:       "struct",
			definition: parser.Definition{Name: "Position", Type: parser.TypeStruct},
			options:    Options{PackageName: "flatdata"},
		},
		{
			name:       "union",
			definition: parser.Definition{Name: "Payload", Type: parser.TypeUnion},
			options:    Options{PackageName: "flatdata"},
		},
		{
			name: "referenced struct",
			definition: parser.Definition{
				Name: "Quality",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "position", Type: "Position", IsStruct: true},
				},
			},
			options: Options{PackageName: "flatdata"},
		},
		{
			name: "referenced union",
			definition: parser.Definition{
				Name: "Quality",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "payload", Type: "Payload", IsUnion: true},
				},
			},
			options: Options{PackageName: "flatdata"},
		},
		{
			name: "unresolved field type",
			definition: parser.Definition{
				Name: "Quality",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "mystery", Type: "MissingType"},
				},
			},
			options: Options{PackageName: "flatdata"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			schema := &parser.Schema{
				FileName:    "Quality",
				Definitions: []parser.Definition{test.definition},
			}
			_, err := Render(schema, test.options)
			if !errors.Is(err, ErrUnsupportedSchema) {
				t.Errorf("Render(%s) error = %v, want errors.Is(ErrUnsupportedSchema)", test.name, err)
			}
		})
	}
}

func TestRenderRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	validSchema := &parser.Schema{FileName: "Quality"}
	tests := []struct {
		name    string
		schema  *parser.Schema
		options Options
	}{
		{name: "nil schema", schema: nil, options: Options{PackageName: "flatdata"}},
		{name: "empty package", schema: validSchema, options: Options{}},
		{name: "keyword package", schema: validSchema, options: Options{PackageName: "func"}},
		{name: "blank identifier package", schema: validSchema, options: Options{PackageName: "_"}},
		{name: "path package", schema: validSchema, options: Options{PackageName: "flat/data"}},
		{name: "empty schema name", schema: &parser.Schema{}, options: Options{PackageName: "flatdata"}},
		{name: "relative schema path", schema: &parser.Schema{FileName: "../quality"}, options: Options{PackageName: "flatdata"}},
		{name: "Windows schema path", schema: &parser.Schema{FileName: `..\quality`}, options: Options{PackageName: "flatdata"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Render(test.schema, test.options); err == nil {
				t.Errorf("Render(%s) error = nil, want non-nil", test.name)
			}
		})
	}
}

func TestRenderConvertsSmallScalars(t *testing.T) {
	t.Parallel()

	schema := &parser.Schema{
		FileName: "Quality",
		Definitions: []parser.Definition{
			{
				Name: "Quality",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "signed_small", Type: "byte"},
					{Name: "small", Type: "short"},
					{Name: "unsigned_small", Type: "ushort"},
					{Name: "smalls", Type: "short", IsVector: true},
				},
			},
		},
	}

	files, err := Render(schema, Options{PackageName: "flatdata"})
	if err != nil {
		t.Fatalf("Render(small scalars) error = %v, want nil", err)
	}
	source := strings.Join(strings.Fields(string(files[0].Content)), " ")
	for _, fragment := range []string{
		"SignedSmall int8",
		"Small int16",
		"UnsignedSmall uint16",
		"Smalls []int16",
		"fbsutils.Convert(t.Small, t.FlatBuffer.TableKey)",
		"fbsutils.Convert(e.Small(), t.FlatBuffer.TableKey)",
		"b.PrependInt16(fbsutils.Convert(t.Smalls[i], t.FlatBuffer.TableKey))",
	} {
		if !strings.Contains(source, fragment) {
			t.Errorf("Render(small scalars) source missing %q; source:\n%s", fragment, files[0].Content)
		}
	}
}

func TestRenderReturnsFormattedSource(t *testing.T) {
	t.Parallel()

	schema := &parser.Schema{
		FileName: "Quality",
		Definitions: []parser.Definition{
			{
				Name: "Quality",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "user_id", Type: "int"},
				},
			},
		},
	}

	files, err := Render(schema, Options{PackageName: "flatdata"})
	if err != nil {
		t.Fatalf("Render(valid table) error = %v, want nil", err)
	}
	if len(files) != 1 {
		t.Fatalf("Render(valid table) file count = %d, want 1", len(files))
	}
	if got, want := files[0].Name, "quality_dto.go"; got != want {
		t.Errorf("Render(valid table) filename = %q, want %q", got, want)
	}

	file, err := goParser.ParseFile(token.NewFileSet(), files[0].Name, files[0].Content, 0)
	if err != nil {
		t.Fatalf("ParseFile(Render(valid table)) error = %v, want nil", err)
	}
	if !hasStructField(file, "QualityDto", "UserID") {
		t.Errorf("Render(valid table) did not generate QualityDto.UserID; source:\n%s", files[0].Content)
	}
}

func TestRenderMatchesQualityGoldenContract(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("..", "testdata", "schemas", "quality.fbs")
	schema, err := parser.ParseFile(testContext(t), schemaPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) error = %v, want nil", schemaPath, err)
	}
	files, err := Render(schema, Options{PackageName: "flatdata"})
	if err != nil {
		t.Fatalf("Render(%q) error = %v, want nil", schemaPath, err)
	}
	if len(files) != 1 {
		t.Fatalf("Render(%q) file count = %d, want 1", schemaPath, len(files))
	}

	golden := loadRenderGolden(t)
	if got := files[0].Name; got != golden.FileName {
		t.Errorf("Render(%q) filename = %q, want %q", schemaPath, got, golden.FileName)
	}
	source := string(files[0].Content)
	normalizedSource := strings.Join(strings.Fields(source), " ")
	for _, fragment := range golden.RequiredFragments {
		normalizedFragment := strings.Join(strings.Fields(fragment), " ")
		if !strings.Contains(normalizedSource, normalizedFragment) {
			t.Errorf("Render(%q) source missing golden fragment %q", schemaPath, fragment)
		}
	}
	for _, fragment := range golden.ForbiddenFragments {
		if strings.Contains(source, fragment) {
			t.Errorf("Render(%q) source contains forbidden golden fragment %q", schemaPath, fragment)
		}
	}
}

func TestRenderSchemasShareOnePackageWithoutRedeclaration(t *testing.T) {
	t.Parallel()

	makeSchema := func(name string) *parser.Schema {
		return &parser.Schema{
			FileName: name,
			Definitions: []parser.Definition{
				{
					Name: name,
					Type: parser.TypeTable,
					Fields: []parser.Field{
						{Name: "label", Type: "string", IsString: true},
					},
				},
			},
		}
	}

	declared := map[string]string{}
	for _, schema := range []*parser.Schema{makeSchema("First"), makeSchema("Second")} {
		files, err := Render(schema, Options{PackageName: "flatdata"})
		if err != nil {
			t.Fatalf("Render(%q) error = %v, want nil", schema.FileName, err)
		}
		for _, generated := range files {
			file, err := goParser.ParseFile(token.NewFileSet(), generated.Name, generated.Content, 0)
			if err != nil {
				t.Fatalf("ParseFile(%q) error = %v, want nil", generated.Name, err)
			}
			for _, name := range packageLevelNames(file) {
				if previous, exists := declared[name]; exists {
					t.Errorf("Render generated package-level %q in both %q and %q", name, previous, generated.Name)
					continue
				}
				declared[name] = generated.Name
			}
		}
	}
}

func packageLevelNames(file *goast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *goast.FuncDecl:
			if decl.Recv == nil {
				names = append(names, decl.Name.Name)
			}
		case *goast.GenDecl:
			for _, specification := range decl.Specs {
				switch spec := specification.(type) {
				case *goast.TypeSpec:
					names = append(names, spec.Name.Name)
				case *goast.ValueSpec:
					for _, identifier := range spec.Names {
						names = append(names, identifier.Name)
					}
				}
			}
		}
	}
	return names
}

func TestRenderHandlesNestedTablesSafely(t *testing.T) {
	t.Parallel()

	schema := &parser.Schema{
		FileName: "Parent",
		Definitions: []parser.Definition{
			{
				Name: "Parent",
				Type: parser.TypeTable,
				Fields: []parser.Field{
					{Name: "child", Type: "Child", IsTable: true},
					{Name: "children", Type: "Child", IsVector: true, IsTable: true},
				},
			},
			{Name: "Child", Type: parser.TypeTable},
		},
	}

	files, err := Render(schema, Options{PackageName: "flatdata"})
	if err != nil {
		t.Fatalf("Render(nested tables) error = %v, want nil", err)
	}
	source := string(files[0].Content)
	if strings.Contains(source, "__") {
		t.Errorf("Render(nested tables) generated double-underscore locals; source:\n%s", source)
	}

	wantedFragments := []string{
		"child := e.Child(nil)",
		"if child != nil {",
		"if err := t.Child.UnmarshalMessage(child); err != nil {",
		`return fmt.Errorf("unmarshal child: %w", err)`,
		"if err := t.Children[i].UnmarshalMessage(child); err != nil {",
		`return fmt.Errorf("unmarshal children[%d]: %w", i, err)`,
	}
	for _, fragment := range wantedFragments {
		if !strings.Contains(source, fragment) {
			t.Errorf("Render(nested tables) source missing %q; source:\n%s", fragment, source)
		}
	}
}

func TestGeneratedPackageCompilesAndRoundTrips(t *testing.T) {
	flatcPath, err := exec.LookPath("flatc")
	if err != nil {
		t.Skip("flatc is required for generated-package integration testing")
	}

	moduleDir := t.TempDir()
	schemaPath := filepath.Join("..", "testdata", "schemas", "quality.fbs")
	flatc := exec.CommandContext(testContext(t), flatcPath, "--go", "-o", moduleDir, schemaPath)
	if output, err := flatc.CombinedOutput(); err != nil {
		t.Fatalf("flatc(%q) error = %v, want nil; output:\n%s", schemaPath, err, output)
	}

	schema, err := parser.ParseFile(testContext(t), schemaPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) error = %v, want nil", schemaPath, err)
	}
	packageDir := filepath.Join(moduleDir, "flatdata")
	if err := Generate(schema, "flatdata", packageDir, false); err != nil {
		t.Fatalf("Generate(%q) error = %v, want nil", schemaPath, err)
	}

	writeIntegrationFile(t, filepath.Join(moduleDir, "go.mod"), generatedModule)
	writeIntegrationFile(t, filepath.Join(packageDir, "quality_dto_test.go"), generatedRuntimeTest)
	vectorData, err := os.ReadFile(filepath.Join("..", "testdata", "table_key_vectors.json"))
	if err != nil {
		t.Fatalf("ReadFile(table key vectors) error = %v, want nil", err)
	}
	writeIntegrationFile(t, filepath.Join(packageDir, "table_key_vectors.json"), string(vectorData))

	goTest := exec.CommandContext(testContext(t), "go", "test", "-mod=mod", "./...")
	goTest.Dir = moduleDir
	if output, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("go test generated package error = %v, want nil; output:\n%s", err, output)
	}
}

func TestCLIHelpDocumentsAcceptedFlags(t *testing.T) {
	output, err := runCLI(t, "-h")
	if err != nil {
		t.Fatalf("go run .. -h error = %v, want nil; output:\n%s", err, output)
	}
	for _, fragment := range []string{
		"-i string",
		"-input string",
		"-o string",
		"-output string",
		"-p string",
		"-package string",
		"Skip field encryption and decryption conversions",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("go run .. -h output missing %q; output:\n%s", fragment, output)
		}
	}
}

func TestCLIFailsWhenInputDirectoryHasNoSchemas(t *testing.T) {
	directory := t.TempDir()
	output, err := runCLI(t, "-i", directory, "-o", filepath.Join(directory, "out"))
	if err == nil {
		t.Fatalf("go run .. -i %q error = nil, want non-nil; output:\n%s", directory, output)
	}
	if !strings.Contains(output, "no .fbs files found") {
		t.Errorf("go run .. -i %q output = %q, want no-schema error", directory, output)
	}
}

func runCLI(t *testing.T, arguments ...string) (string, error) {
	t.Helper()

	commandArguments := append([]string{"run", ".."}, arguments...)
	command := exec.CommandContext(testContext(t), "go", commandArguments...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func writeIntegrationFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func loadRenderGolden(t *testing.T) renderGolden {
	t.Helper()

	path := filepath.Join("..", "testdata", "golden", "quality_dto.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	var golden renderGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil", path, err)
	}
	return golden
}

const generatedModule = `module generatedtest

go 1.23.0

require (
	github.com/arisu-archive/bluearchive-fbs-utils v0.0.0-20260823204751-dd41aefb457e
	github.com/google/flatbuffers v25.12.19+incompatible
)
`

const generatedRuntimeTest = `package flatdata

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	fbsutils "github.com/arisu-archive/bluearchive-fbs-utils"
	flatbuffers "github.com/google/flatbuffers/go"
)

type sharedKeyVectors struct {
	KeyDerivation []struct {
		Table       string ` + "`json:\"table\"`" + `
		TableKeyHex string ` + "`json:\"table_key_hex\"`" + `
	} ` + "`json:\"key_derivation\"`" + `
	FieldConversion struct {
		TableKeyHex      string ` + "`json:\"table_key_hex\"`" + `
		WrongChildKeyHex string ` + "`json:\"wrong_child_key_hex\"`" + `
		Int32 struct {
			PlaintextSigned        int32 ` + "`json:\"plaintext_signed\"`" + `
			EncodedSigned          int32 ` + "`json:\"encoded_signed\"`" + `
			WrongChildDecodeSigned int32 ` + "`json:\"wrong_child_decode_signed\"`" + `
		} ` + "`json:\"int32\"`" + `
		String struct {
			Plaintext     string ` + "`json:\"plaintext\"`" + `
			EncodedBase64 string ` + "`json:\"encoded_base64\"`" + `
		} ` + "`json:\"string\"`" + `
	} ` + "`json:\"field_conversion\"`" + `
}

func TestSharedTableKeyVectorsMatchFBSUtils(t *testing.T) {
	data, err := os.ReadFile("table_key_vectors.json")
	if err != nil {
		t.Fatalf("ReadFile(table_key_vectors.json) error = %v, want nil", err)
	}
	var vectors sharedKeyVectors
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("json.Unmarshal(table_key_vectors.json) error = %v, want nil", err)
	}
	for _, vector := range vectors.KeyDerivation {
		if got := hex.EncodeToString(fbsutils.CreateTableKey(vector.Table)); got != vector.TableKeyHex {
			t.Errorf("CreateTableKey(%q) = %q, want %q", vector.Table, got, vector.TableKeyHex)
		}
	}

	conversion := vectors.FieldConversion
	key, err := hex.DecodeString(conversion.TableKeyHex)
	if err != nil {
		t.Fatalf("hex.DecodeString(root key) error = %v, want nil", err)
	}
	wrongKey, err := hex.DecodeString(conversion.WrongChildKeyHex)
	if err != nil {
		t.Fatalf("hex.DecodeString(child key) error = %v, want nil", err)
	}
	if got := fbsutils.Convert(conversion.Int32.EncodedSigned, key); got != conversion.Int32.PlaintextSigned {
		t.Errorf("Convert(encoded int32, root key) = %d, want %d", got, conversion.Int32.PlaintextSigned)
	}
	if got := fbsutils.Convert(conversion.Int32.EncodedSigned, wrongKey); got != conversion.Int32.WrongChildDecodeSigned {
		t.Errorf("Convert(encoded int32, child key) = %d, want %d", got, conversion.Int32.WrongChildDecodeSigned)
	}
	if got := fbsutils.Convert(conversion.String.EncodedBase64, key); got != conversion.String.Plaintext {
		t.Errorf("Convert(encoded string, root key) = %q, want %q", got, conversion.String.Plaintext)
	}
}

func TestQualityDtoRoundTrip(t *testing.T) {
	want := QualityDto{
		UserID:   42,
		Kind:     QualityKindGood,
		Kinds:    []QualityKind{QualityKindUnknown, QualityKindGood},
		Scores:   []int32{7, -3},
		Enabled:  true,
		Child:    ChildDto{Name: "single"},
		Children: []ChildDto{{Name: "first"}, {Name: "second"}},
		Labels:   []string{"alpha", "beta"},
	}

	data, err := want.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v, want nil", err)
	}
	var got QualityDto
	if err := got.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal(Marshal()) error = %v, want nil", err)
	}
	if got.UserID != want.UserID || got.Child.Name != want.Child.Name {
		t.Errorf("Unmarshal(Marshal()) scalar values = (%d, %q), want (%d, %q)", got.UserID, got.Child.Name, want.UserID, want.Child.Name)
	}
	if got.Kind != QualityKindGood || len(got.Kinds) != 2 || got.Kinds[0] != QualityKindUnknown || got.Kinds[1] != QualityKindGood {
		t.Errorf("Unmarshal(Marshal()) enum values = (%v, %v), want (Good, [Unknown Good])", got.Kind, got.Kinds)
	}
	if len(got.Scores) != 2 || got.Scores[0] != 7 || got.Scores[1] != -3 || !got.Enabled {
		t.Errorf("Unmarshal(Marshal()) primitive values = (scores: %v, enabled: %t), want ([7 -3], true)", got.Scores, got.Enabled)
	}
	if len(got.Children) != 2 || got.Children[0].Name != "first" || got.Children[1].Name != "second" {
		t.Errorf("Unmarshal(Marshal()) children = %+v, want first and second", got.Children)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "alpha" || got.Labels[1] != "beta" {
		t.Errorf("Unmarshal(Marshal()) labels = %q, want [alpha beta]", got.Labels)
	}
}

func TestQualityDtoAllowsMissingChild(t *testing.T) {
	builder := flatbuffers.NewBuilder(0)
	QualityStart(builder)
	root := QualityEnd(builder)
	builder.Finish(root)

	dto := QualityDto{Child: ChildDto{Name: "stale"}}
	if err := dto.Unmarshal(builder.FinishedBytes()); err != nil {
		t.Errorf("Unmarshal(table without child) error = %v, want nil", err)
	}
	if dto.Child.Name != "" {
		t.Errorf("Unmarshal(table without child) child name = %q, want empty", dto.Child.Name)
	}
}
`

func hasStructField(file *goast.File, structName, fieldName string) bool {
	for _, declaration := range file.Decls {
		typeDeclaration, ok := declaration.(*goast.GenDecl)
		if !ok || typeDeclaration.Tok != token.TYPE {
			continue
		}
		for _, specification := range typeDeclaration.Specs {
			typeSpecification, ok := specification.(*goast.TypeSpec)
			if !ok || typeSpecification.Name.Name != structName {
				continue
			}
			structure, ok := typeSpecification.Type.(*goast.StructType)
			if !ok {
				return false
			}
			for _, field := range structure.Fields.List {
				if len(field.Names) == 1 && field.Names[0].Name == fieldName {
					return true
				}
			}
		}
	}
	return false
}
