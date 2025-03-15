package generator

import (
	"strings"

	. "github.com/dave/jennifer/jen"

	"github.com/iancoleman/strcase"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

// Helper functions shared across generator files

// toExportedName converts a field name to an exported Go identifier
func toExportedName(name string) string {
	return strcase.ToCamel(strings.ReplaceAll(name, "Excel", ""))
}

// getDefTypeStr returns a string representation of the definition type
func getDefTypeStr(defType parser.SchemaType) string {
	switch defType {
	case parser.TypeStruct:
		return "struct"
	case parser.TypeTable:
		return "table"
	case parser.TypeEnum:
		return "enum"
	case parser.TypeUnion:
		return "union"
	default:
		return "unknown"
	}
}

// getGoType converts a FlatBuffers type to a Go type expression
func getGoType(field parser.Field) *Statement {
	// Handle vector types
	if field.IsVector {
		return Index().Add(getBaseGoType(field))
	}

	return getBaseGoType(field)
}

// getBaseGoType returns the Go type for a FlatBuffers type
func getBaseGoType(field parser.Field) *Statement {
	// Handle special types
	if field.IsString {
		return String()
	}

	if field.IsStruct || field.IsUnion {
		return Op("*").Id(field.Type)
	}

	if field.IsEnum {
		return Id(field.Type)
	}

	// Handle primitive types
	switch field.Type {
	case "bool":
		return Bool()
	case "byte", "ubyte":
		return Byte()
	case "short":
		return Int16()
	case "ushort":
		return Uint16()
	case "int":
		return Int32()
	case "uint":
		return Uint32()
	case "long":
		return Int64()
	case "ulong":
		return Uint64()
	case "float":
		return Float32()
	case "double":
		return Float64()
	default:
		// Default to interface{} for unknown types
		return Id(toExportedName(field.Type))
	}
}
