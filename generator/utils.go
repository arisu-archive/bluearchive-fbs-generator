package generator

import (
	"strings"
	"unicode"

	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func toCamel(name string) string {
	if name == "" {
		return ""
	}

	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == ' ' || r == '-'
	})
	var result strings.Builder
	for _, word := range words {
		upperWord := strings.ToUpper(word)
		if isInitialism(upperWord) {
			result.WriteString(upperWord)
			continue
		}

		runes := []rune(word)
		result.WriteRune(unicode.ToUpper(runes[0]))
		result.WriteString(string(runes[1:]))
	}
	return result.String()
}

func toFlatBuffersName(name string) string {
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

func isInitialism(word string) bool {
	switch word {
	case "ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON", "QPS", "RAM", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI", "URL", "UTF8", "VM", "XML", "XMPP", "XSRF", "XSS":
		return true
	default:
		return false
	}
}

func toExportedName(name string) string {
	return toCamel(name)
}

func toLocalName(name string) string {
	exportedName := toExportedName(name)
	if exportedName == "" {
		return ""
	}
	runes := []rune(exportedName)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func toModelName(name string) string {
	return toExportedName(name) + "Dto"
}

func toFileName(name string) string {
	return strcase.ToSnake(toModelName(name)) + ".go"
}

func getGoType(field parser.Field) *jen.Statement {
	if field.IsVector {
		return jen.Index().Add(getBaseGoType(field))
	}
	return getBaseGoType(field)
}

func getBaseGoType(field parser.Field) *jen.Statement {
	if field.IsString {
		return jen.String()
	}
	if field.IsEnum {
		return jen.Id(field.Type)
	}

	switch field.Type {
	case "bool":
		return jen.Bool()
	case "byte":
		return jen.Int8()
	case "ubyte":
		return jen.Uint8()
	case "short":
		return jen.Int16()
	case "ushort":
		return jen.Uint16()
	case "int":
		return jen.Int32()
	case "uint":
		return jen.Uint32()
	case "long":
		return jen.Int64()
	case "ulong":
		return jen.Uint64()
	case "float":
		return jen.Float32()
	case "double":
		return jen.Float64()
	default:
		if field.IsNested() {
			return jen.Id(toModelName(field.Type))
		}
		return jen.Id(field.Type)
	}
}
