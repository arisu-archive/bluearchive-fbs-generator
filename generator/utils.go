package generator

import (
	"strings"
	"unicode"

	. "github.com/dave/jennifer/jen"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func toCamel(name string) string {
	if name == "" {
		return ""
	}

	var result strings.Builder
	upperNext := true

	for _, r := range name {
		switch {
		case r == '_' || r == ' ' || r == '-':
			upperNext = true
		case upperNext:
			result.WriteRune(unicode.ToUpper(r))
			upperNext = false
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// toExportedName converts a field name to an exported Go identifier.
func toExportedName(name string) string {
	// It is camel but DO NOT make the character after number uppercase
	return toCamel(strings.ReplaceAll(name, "Excel", ""))
}

func toModelName(name string) string {
	return toExportedName(name) + "Dto"
}

// getDefTypeStr returns a string representation of the definition type.
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
		// TODO: Use more generic method to handle struct references
		if !field.IsPrimitive() && strings.Contains(field.Name, "data_list") {
			return Id(toModelName(field.Type))
		}
		return Id(field.Type)
	}
}
