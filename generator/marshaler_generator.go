package generator

import (
	"strings"

	. "github.com/dave/jennifer/jen"

	"github.com/iancoleman/strcase"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

// generateMarshalMessage is a helper function that generates code for the MarshalMessage method.
// Function Signature:
// func (t *<modelName>) Marshal(b *flatbuffers.Builder) flatbuffers.UOffsetT
func generateMarshalMessage(f *File, def parser.Definition, modelName string, withoutDecryption bool) {
	fieldSetter := func(g *Group, field parser.Field, fieldAccessor Code) {
		// <FlatBufferMessage>Add<FieldName>(b, <fieldAccessor>)
		g.Id(def.Name+"Add"+toExportedName(field.Name)).Call(Id("b"), fieldAccessor)
	}

	f.Comment("MarshalModel marshals the struct into flatbuffers offset")
	f.Func().Params(Id("t").Op("*").Id(modelName)).Id("MarshalModel").Params(
		Id("b").Op("*").Qual("github.com/google/flatbuffers/go", "Builder"),
	).Qual("github.com/google/flatbuffers/go", "UOffsetT").BlockFunc(func(g *Group) {
		// Skip the initialization of the table key if withoutDecryption is true.
		if !withoutDecryption {
			g.If(Id("t").Dot("FlatBuffer").Dot("TableKey").Op("==").Nil()).Block(
				Id("t").Dot("FlatBuffer").Dot("InitKey").Call(
					Qual("github.com/arisu-archive/bluearchive-fbs-utils", "CreateTableKey").Call(
						Lit(
							strings.ReplaceAll(strings.ReplaceAll(def.Name, "ExcelTable", ""), "Excel", ""),
						),
					),
				),
			)
		}

		// Pre-create all strings and nested objects BEFORE calling Start
		offsetVarIndex := 0
		for _, field := range def.Fields {
			fieldAccessor := Id("t").Dot(toExportedName(field.Name))
			offsetVarName := "__offset_" + field.Name

			if field.IsVector {
				if field.IsString || field.IsNested() {
					// Create offset variable for vector of strings or nested objects
					g.Var().Id(offsetVarName).Qual("github.com/google/flatbuffers/go", "UOffsetT")
				}

				if field.IsString {
					// Pre-create vector of strings
					fieldLength := Id("len").Call(fieldAccessor)
					g.Id("__stringOffsets_"+field.Name).Op(":=").Make(Index().Qual("github.com/google/flatbuffers/go", "UOffsetT"), fieldLength)
					g.For(Id("i").Op(":=").Range().Add(fieldLength)).Block(
						Id("__stringOffsets_" + field.Name).Index(Id("i")).Op("=").Id("b").Dot("CreateString").Call(
							fieldConverter(fieldAccessor.Clone().Index(Id("i"))),
						),
					)
					// Create the vector
					g.Id(def.Name+"Start"+toExportedName(field.Name)+"Vector").Call(Id("b"), fieldLength)
					g.For(Id("i").Op(":=").Range().Add(fieldLength)).Block(
						Id("b").Dot("PrependUOffsetT").Call(
							Id("__stringOffsets_" + field.Name).Index(fieldLength.Clone().Op("-").Id("i").Op("-").Lit(1)),
						),
					)
					g.Id(offsetVarName).Op("=").Id("b").Dot("EndVector").Call(fieldLength)
					offsetVarIndex++
				} else if field.IsNested() {
					// Pre-create vector of nested objects
					fieldLength := Id("len").Call(fieldAccessor)
					g.Id("__nestedOffsets_"+field.Name).Op(":=").Make(Index().Qual("github.com/google/flatbuffers/go", "UOffsetT"), fieldLength)
					g.For(Id("i").Op(":=").Range().Add(fieldLength)).BlockFunc(func(g *Group) {
						indexedFieldAccessor := fieldAccessor.Clone().Index(Id("i"))
						if !withoutDecryption {
							g.Add(indexedFieldAccessor.Clone().Dot("InitKey").Call(Id("t").Dot("FlatBuffer").Dot("TableKey")))
						}
						g.Id("__nestedOffsets_" + field.Name).Index(Id("i")).Op("=").Add(indexedFieldAccessor.Clone().Dot("MarshalModel").Call(Id("b")))
					})
					// Create the vector (in reverse order)
					g.Id(def.Name+"Start"+toExportedName(field.Name)+"Vector").Call(Id("b"), fieldLength)
					g.For(Id("i").Op(":=").Range().Add(fieldLength)).Block(
						Id("b").Dot("PrependUOffsetT").Call(
							Id("__nestedOffsets_" + field.Name).Index(fieldLength.Clone().Op("-").Id("i").Op("-").Lit(1)),
						),
					)
					g.Id(offsetVarName).Op("=").Id("b").Dot("EndVector").Call(fieldLength)
					offsetVarIndex++
				}
			} else if field.IsNested() {
				// Pre-create single nested object
				if !withoutDecryption {
					g.Add(fieldAccessor.Clone().Dot("InitKey").Call(Id("t").Dot("FlatBuffer").Dot("TableKey")))
				}
				g.Id(offsetVarName).Op(":=").Add(fieldAccessor.Clone().Dot("MarshalModel").Call(Id("b")))
				offsetVarIndex++
			} else if field.IsString {
				// Pre-create single string
				g.Id(offsetVarName).Op(":=").Id("b").Dot("CreateString").Call(fieldConverter(fieldAccessor))
				offsetVarIndex++
			}
		}

		// <FlatBufferMessage>Start(b)
		g.Id(def.Name + "Start").Call(Id("b"))

		// Start adding codes to marshal the struct fields.
		for _, field := range def.Fields {
			fieldAccessor := Id("t").Dot(toExportedName(field.Name))
			offsetVarName := "__offset_" + field.Name

			if field.IsVector {
				if field.IsString || field.IsNested() {
					// Use pre-created offset
					fieldSetter(g, field, Id(offsetVarName))
				} else {
					// Handle primitive vectors inline
					fieldLength := Id("len").Call(fieldAccessor)
					g.Id(def.Name+"Start"+toExportedName(field.Name)+"Vector").Call(Id("b"), fieldLength)
					g.For(Id("i").Op(":=").Range().Add(fieldLength)).BlockFunc(func(g *Group) {
						indexedFieldAccessor := fieldAccessor.Clone().Index(fieldLength.Clone().Op("-").Id("i").Op("-").Lit(1))
						goType := strcase.ToCamel(getBaseGoType(field).GoString())

						if field.IsEnum {
							goType = "Int32"
							indexedFieldAccessor = fieldConverter(Id("int32").Call(indexedFieldAccessor))
						} else if field.IsPrimitive() && field.Type != "bool" {
							indexedFieldAccessor = fieldConverter(indexedFieldAccessor)
						}

						g.Id("b").Dot("Prepend" + goType).Call(indexedFieldAccessor)
					})
					fieldSetter(g, field, Id("b").Dot("EndVector").Call(fieldLength))
				}
			} else if field.IsNested() {
				// Use pre-created nested object offset
				fieldSetter(g, field, Id(offsetVarName))
			} else if field.IsString {
				// Use pre-created string offset
				fieldSetter(g, field, Id(offsetVarName))
			} else {
				// Handle primitive fields inline
				if field.Type != "bool" {
					fieldAccessor = fieldConverter(fieldAccessor)
				}
				fieldSetter(g, field, fieldAccessor)
			}
		}
		g.Return(Id(def.Name + "End").Call(Id("b")))
	})

	f.Line()
}

// generateMarshal is a helper function that generates code for the Marshal method.
// Function Signature:
// func (t *<modelName>) Marshal(b *flatbuffers.Builder) flatbuffers.UOffsetT
func generateMarshal(f *File, def parser.Definition, modelName string) {
	f.Comment("Marshal marshals the struct into a FlatBuffers buffer")
	f.Func().Params(Id("t").Op("*").Id(modelName)).Id("Marshal").Params().Params(
		Index().Add(Byte()),
		Error(),
	).BlockFunc(func(g *Group) {
		g.Id("b").Op(":=").Qual("github.com/google/flatbuffers/go", "NewBuilder").Call(Lit(0))
		g.Id("b").Dot("Finish").Call(
			Id("t").Dot("MarshalModel").Call(Id("b")),
		)
		g.Return(
			Id("b").Dot("FinishedBytes").Call(),
			Nil(),
		)
	})
	f.Line()
}

// generateStructMarshaler creates a Go marshaler for a FlatBuffers struct.
func generateStructMarshaler(f *File, def parser.Definition, withoutDecryption bool) {
	modelName := toModelName(def.Name)
	generateMarshalMessage(f, def, modelName, withoutDecryption)
	generateMarshal(f, def, modelName)
}
