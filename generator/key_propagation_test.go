package generator

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/format"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"unicode/utf16"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

type keyContract struct {
	KeyScope          string `json:"key_scope"`
	RootDefault       string `json:"root_default"`
	ExplicitRootKey   string `json:"explicit_root_key"`
	NestedKey         string `json:"nested_key"`
	StandaloneKey     string `json:"standalone_key"`
	WithoutDecryption string `json:"without_decryption"`
	WholeBufferXOR    string `json:"whole_buffer_xor"`
}

type keyDerivationVector struct {
	Table       string `json:"table"`
	XXHash32Hex string `json:"xxhash32_hex"`
	TableKeyHex string `json:"table_key_hex"`
}

type int32ConversionVector struct {
	PlaintextSigned        int32 `json:"plaintext_signed"`
	EncodedSigned          int32 `json:"encoded_signed"`
	WrongChildDecodeSigned int32 `json:"wrong_child_decode_signed"`
}

type stringConversionVector struct {
	Plaintext           string `json:"plaintext"`
	PlaintextUTF16LEHex string `json:"plaintext_utf16le_hex"`
	EncodedHex          string `json:"encoded_hex"`
	EncodedBase64       string `json:"encoded_base64"`
}

type fieldConversionVector struct {
	TableKeyHex      string                 `json:"table_key_hex"`
	WrongChildKeyHex string                 `json:"wrong_child_key_hex"`
	Int32            int32ConversionVector  `json:"int32"`
	String           stringConversionVector `json:"string"`
}

type tableKeyVectors struct {
	SchemaVersion   int                   `json:"schema_version"`
	Contract        keyContract           `json:"contract"`
	KeyDerivation   []keyDerivationVector `json:"key_derivation"`
	FieldConversion fieldConversionVector `json:"field_conversion"`
}

func TestGeneratePropagatesTableKeyToNestedUnmarshal(t *testing.T) {
	t.Parallel()

	file := generateNestedModel(t, false /* withoutDecryption */)
	method := findMethod(t, file, "ParentDto", "UnmarshalMessage")
	gotPropagated, gotNested := nestedKeyPropagation(t, method.Body)

	const wantNested = 2
	if gotNested != wantNested || gotPropagated != wantNested {
		t.Errorf(
			"Generate(parent with singular and vector children) propagated %d of %d nested keys, want %d of %d; generated method:\n%s",
			gotPropagated,
			gotNested,
			wantNested,
			wantNested,
			nodeString(t, method.Body),
		)
	}
}

func TestGenerateDTOsPreserveExplicitTableKeys(t *testing.T) {
	t.Parallel()

	file := generateNestedModel(t, false /* withoutDecryption */)
	tests := []struct {
		name         string
		receiverName string
		wantKeyName  string
	}{
		{name: "parent root", receiverName: "ParentDto", wantKeyName: "Parent"},
		{name: "standalone child", receiverName: "ChildDto", wantKeyName: "Child"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			method := findMethod(t, file, test.receiverName, "UnmarshalMessage")
			got := guardedDefaultTableKeyNames(t, method.Body)
			want := []string{test.wantKeyName}
			if len(got) != len(want) || got[0] != want[0] {
				t.Errorf("Generate(%s) nil-guarded CreateTableKey names = %q, want %q", test.receiverName, got, want)
			}
		})
	}
}

func TestGenerateWithoutDecryptionOmitsTableKeyOperations(t *testing.T) {
	t.Parallel()

	file := generateNestedModel(t, true /* withoutDecryption */)
	gotInitKey := countSelectorCalls(file, "InitKey")
	gotCreateTableKey := countSelectorCalls(file, "CreateTableKey")

	if gotInitKey != 0 || gotCreateTableKey != 0 {
		t.Errorf(
			"Generate(without decryption) key operation counts = (InitKey: %d, CreateTableKey: %d), want (InitKey: 0, CreateTableKey: 0)",
			gotInitKey,
			gotCreateTableKey,
		)
	}
}

func TestTableKeyVectorsArePortable(t *testing.T) {
	t.Parallel()

	vectors := loadTableKeyVectors(t)
	wantContract := keyContract{
		KeyScope:          "dto_tree",
		RootDefault:       "derive_from_root_table_name",
		ExplicitRootKey:   "preserve",
		NestedKey:         "inherit_parent",
		StandaloneKey:     "derive_from_own_table_name",
		WithoutDecryption: "no_key_initialization_or_propagation",
		WholeBufferXOR:    "separate_preprocessing_step",
	}
	if vectors.SchemaVersion != 1 || vectors.Contract != wantContract {
		t.Errorf(
			"loadTableKeyVectors() = (schema version: %d, contract: %+v), want (schema version: 1, contract: %+v)",
			vectors.SchemaVersion,
			vectors.Contract,
			wantContract,
		)
	}

	wantDerivations := []keyDerivationVector{
		{Table: "AnimatorDataTable", XXHash32Hex: "f22d1b4f", TableKeyHex: "0d2add2e63844b29"},
		{Table: "AnimatorData", XXHash32Hex: "68943335", TableKeyHex: "1609ee0e2cbed631"},
		{Table: "AniStateData", XXHash32Hex: "c4353346", TableKeyHex: "20d2e96da7695a2a"},
		{Table: "AniEventData", XXHash32Hex: "a660abe9", TableKeyHex: "ec148066c5b8f867"},
	}
	if len(vectors.KeyDerivation) != len(wantDerivations) {
		t.Fatalf("loadTableKeyVectors() derivation count = %d, want %d", len(vectors.KeyDerivation), len(wantDerivations))
	}
	for i, want := range wantDerivations {
		got := vectors.KeyDerivation[i]
		if got != want {
			t.Errorf("loadTableKeyVectors() derivation %d = %+v, want %+v", i, got, want)
		}
		if gotKeyLength := len(mustDecodeHex(t, got.TableKeyHex)); gotKeyLength != 8 {
			t.Errorf("loadTableKeyVectors() key length for %q = %d, want 8", got.Table, gotKeyLength)
		}
	}
	if got := vectors.FieldConversion.TableKeyHex; got != wantDerivations[0].TableKeyHex {
		t.Errorf("loadTableKeyVectors() field-conversion root key = %q, want %q", got, wantDerivations[0].TableKeyHex)
	}
	if got := vectors.FieldConversion.WrongChildKeyHex; got != wantDerivations[3].TableKeyHex {
		t.Errorf("loadTableKeyVectors() field-conversion child key = %q, want %q", got, wantDerivations[3].TableKeyHex)
	}

	rootKey := mustDecodeHex(t, vectors.FieldConversion.TableKeyHex)
	childKey := mustDecodeHex(t, vectors.FieldConversion.WrongChildKeyHex)

	plainInt := make([]byte, 4)
	binary.LittleEndian.PutUint32(plainInt, uint32(vectors.FieldConversion.Int32.PlaintextSigned))
	encodedInt := xorWithKey(plainInt, rootKey)
	gotEncodedInt := int32(binary.LittleEndian.Uint32(encodedInt))
	if want := vectors.FieldConversion.Int32.EncodedSigned; gotEncodedInt != want {
		t.Errorf("xorWithKey(int32 %d, root key) = %d, want %d", vectors.FieldConversion.Int32.PlaintextSigned, gotEncodedInt, want)
	}

	wronglyDecodedInt := xorWithKey(encodedInt, childKey)
	gotWronglyDecodedInt := int32(binary.LittleEndian.Uint32(wronglyDecodedInt))
	if want := vectors.FieldConversion.Int32.WrongChildDecodeSigned; gotWronglyDecodedInt != want {
		t.Errorf("xorWithKey(encoded int32 %d, child key) = %d, want %d", gotEncodedInt, gotWronglyDecodedInt, want)
	}

	stringVector := vectors.FieldConversion.String
	if stringVector.Plaintext == "" || stringVector.PlaintextUTF16LEHex == "" || stringVector.EncodedHex == "" || stringVector.EncodedBase64 == "" {
		t.Fatalf("loadTableKeyVectors() string vector = %+v, want all fields non-empty", stringVector)
	}
	plainString := utf16LE(stringVector.Plaintext)
	if got := hex.EncodeToString(plainString); got != stringVector.PlaintextUTF16LEHex {
		t.Errorf(
			"utf16LE(%q) = %q, want %q",
			stringVector.Plaintext,
			got,
			stringVector.PlaintextUTF16LEHex,
		)
	}

	encodedString := xorWithKey(plainString, rootKey)
	if got := hex.EncodeToString(encodedString); got != stringVector.EncodedHex {
		t.Errorf("xorWithKey(%q, root key) hex = %q, want %q", stringVector.Plaintext, got, stringVector.EncodedHex)
	}
	if got := base64.StdEncoding.EncodeToString(encodedString); got != stringVector.EncodedBase64 {
		t.Errorf("xorWithKey(%q, root key) base64 = %q, want %q", stringVector.Plaintext, got, stringVector.EncodedBase64)
	}
}

func generateNestedModel(t *testing.T, withoutDecryption bool) *ast.File {
	t.Helper()

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

	outputDir := t.TempDir()
	if err := Generate(schema, "flatdata", outputDir, withoutDecryption); err != nil {
		t.Fatalf("Generate(nested schema, withoutDecryption=%t): %v", withoutDecryption, err)
	}

	path := filepath.Join(outputDir, "ParentDto.go")
	file, err := goparser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse generated model %q: %v", path, err)
	}

	return file
}

func findMethod(t *testing.T, file *ast.File, receiverName, methodName string) *ast.FuncDecl {
	t.Helper()

	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok || method.Name.Name != methodName || method.Recv == nil {
			continue
		}
		if methodReceiverName(method) == receiverName {
			return method
		}
	}

	t.Fatalf("findMethod(receiver=%q, method=%q) found no method", receiverName, methodName)
	return nil
}

func methodReceiverName(method *ast.FuncDecl) string {
	receiver := method.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, _ := receiver.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

func nestedKeyPropagation(t *testing.T, body *ast.BlockStmt) (int, int) {
	t.Helper()

	var propagated int
	var nested int
	ast.Inspect(body, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}

		for i, statement := range block.List {
			unmarshalCall, unmarshalReceiver, ok := selectorCall(statement, "UnmarshalMessage")
			if !ok {
				continue
			}
			nested++
			unmarshalReceiverName := nodeString(t, unmarshalReceiver)
			var wantArgument string
			switch unmarshalReceiverName {
			case "t.Child":
				wantArgument = "e.Child(nil)"
			case "t.Children[i]":
				wantArgument = "d"
			default:
				continue
			}
			if len(unmarshalCall.Args) != 1 || nodeString(t, unmarshalCall.Args[0]) != wantArgument {
				continue
			}
			if i == 0 {
				continue
			}

			initCall, initReceiver, ok := selectorCall(block.List[i-1], "InitKey")
			if !ok || nodeString(t, initReceiver) != unmarshalReceiverName {
				continue
			}
			if len(initCall.Args) != 1 || nodeString(t, initCall.Args[0]) != "t.FlatBuffer.TableKey" {
				continue
			}
			propagated++
		}

		return true
	})

	return propagated, nested
}

func selectorCall(statement ast.Stmt, selectorName string) (*ast.CallExpr, ast.Expr, bool) {
	expressionStatement, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil, nil, false
	}
	call, ok := expressionStatement.X.(*ast.CallExpr)
	if !ok {
		return nil, nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return nil, nil, false
	}
	return call, selector.X, true
}

func nodeString(t *testing.T, node ast.Node) string {
	t.Helper()

	var output bytes.Buffer
	if err := format.Node(&output, token.NewFileSet(), node); err != nil {
		t.Fatalf("format generated syntax: %v", err)
	}
	return output.String()
}

func guardedDefaultTableKeyNames(t *testing.T, node ast.Node) []string {
	t.Helper()

	names := []string{}
	ast.Inspect(node, func(node ast.Node) bool {
		guard, ok := node.(*ast.IfStmt)
		if !ok || nodeString(t, guard.Cond) != "t.FlatBuffer.TableKey == nil" {
			return true
		}

		for _, statement := range guard.Body.List {
			call, receiver, ok := selectorCall(statement, "InitKey")
			if !ok || nodeString(t, receiver) != "t.FlatBuffer" || len(call.Args) != 1 {
				continue
			}
			name, ok := createTableKeyName(t, call.Args[0])
			if ok {
				names = append(names, name)
			}
		}
		return false
	})
	return names
}

func createTableKeyName(t *testing.T, expression ast.Expr) (string, bool) {
	t.Helper()

	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "CreateTableKey" {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return "", false
	}
	name, err := strconv.Unquote(literal.Value)
	if err != nil {
		t.Fatalf("unquote CreateTableKey argument %q: %v", literal.Value, err)
	}
	return name, true
}

func countSelectorCalls(node ast.Node, selectorName string) int {
	var count int
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == selectorName {
			count++
		}
		return true
	})
	return count
}

func loadTableKeyVectors(t *testing.T) tableKeyVectors {
	t.Helper()

	path := filepath.Join("..", "testdata", "table_key_vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read table key vectors %q: %v", path, err)
	}
	var vectors tableKeyVectors
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("unmarshal table key vectors %q: %v", path, err)
	}
	return vectors
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q): %v", value, err)
	}
	if len(decoded) == 0 {
		t.Fatalf("hex.DecodeString(%q) returned an empty key", value)
	}
	return decoded
}

func xorWithKey(data, key []byte) []byte {
	result := make([]byte, len(data))
	for i, value := range data {
		result[i] = value ^ key[i%len(key)]
	}
	return result
}

func utf16LE(value string) []byte {
	codeUnits := utf16.Encode([]rune(value))
	result := make([]byte, len(codeUnits)*2)
	for i, codeUnit := range codeUnits {
		binary.LittleEndian.PutUint16(result[i*2:], codeUnit)
	}
	return result
}
