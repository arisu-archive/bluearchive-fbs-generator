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
func generateMarshalMessage(f *File, def parser.Definition, modelName string) {
	fieldSetter := func(g *Group, field parser.Field, fieldAccessor Code) {
		// <FlatBufferMessage>Add<FieldName>(b, <fieldAccessor>)
		g.Id(def.Name+"Add"+toExportedName(field.Name)).Call(Id("b"), fieldAccessor)
	}

	f.Comment("MarshalModel marshals the struct into flatbuffers offset")
	f.Func().Params(Id("t").Op("*").Id(modelName)).Id("MarshalModel").Params(
		Id("b").Op("*").Qual("github.com/google/flatbuffers/go", "Builder"),
	).Qual("github.com/google/flatbuffers/go", "UOffsetT").BlockFunc(func(g *Group) {
		g.If(Id("t").Dot("FlatBuffer").Dot("TableKey").Op("==").Nil()).Block(
			Id("t").Dot("FlatBuffer").Dot("InitKey").Call(
				Qual("github.com/arisu-archive/bluearchive-fbs-utils", "CreateTableKey").Call(
					Lit(
						strings.ReplaceAll(strings.ReplaceAll(def.Name, "Excel", ""), "ExcelTable", ""),
					),
				),
			),
		)
		// <FlatBufferMessage>Start(b)
		g.Id(def.Name + "Start").Call(Id("b"))
		// Start adding codes to unmarshal the struct fields.
		for _, field := range def.Fields {
			fieldAccessor := Id("t").Dot(toExportedName(field.Name))
			if field.IsVector {
				fieldLength := Id("len").Call(fieldAccessor)
				// <FlatBufferMessage>Start<FieldName>Vector(b, len(t.FieldName))
				g.Id(def.Name+"Start"+toExportedName(field.Name)+"Vector").Call(Id("b"), fieldLength)
				g.For(Id("i").Op(":=").Range().Add(fieldLength)).BlockFunc(func(g *Group) {
					// If the field is a nested struct, we need to marshal it first.
					indexedFieldAccessor := fieldAccessor.Clone().Index(fieldLength.Clone().Op("-").Id("i").Op("-").Lit(1))
					if field.IsNested() {
						// b.PrependUOffsetT(t.FieldName[i].MarshalModel(b))
						g.Comment("The array should be reversed.")
						g.Id("b").Dot("PrependUOffsetT").Call(indexedFieldAccessor.Dot("MarshalModel").Call(Id("b")))
						return
					}

					// b.Prepend<GoType>(fbsutils.Convert(t.FieldName[i], t.FlatBuffer.TableKey))
					goType := strcase.ToCamel(getBaseGoType(field).GoString())
					switch true {
					case field.IsString:
						goType = "UOffsetT"
						indexedFieldAccessor = Id("b").Dot("CreateString").Call(indexedFieldAccessor)
					case field.IsEnum:
						goType = "Int32"
						indexedFieldAccessor = Id("int32").Call(fieldConverter(indexedFieldAccessor))
					}
					g.Id("b").Dot("Prepend" + goType).Call(fieldConverter(indexedFieldAccessor))
				})
				// <FlatBufferMessage>Add<FieldName>(b, b.EndVector(len(t.FieldName)))
				fieldSetter(g, field, Id("b").Dot("EndVector").Call(Id("len").Call(fieldAccessor)))
			} else if field.IsNested() {
				// <FlatBufferMessage>Add<FieldName>(b, <fieldAccessor>.MarshalModel(b))
				fieldSetter(g, field, fieldAccessor.Dot("MarshalModel").Call(Id("b")))
			} else {
				if field.IsString {
					fieldAccessor = Id("b").Dot("CreateString").Call(fieldAccessor)
				}
				// <FlatBufferMessage>Add<FieldName>(b, fbsutils.Convert(<FieldName>, t.FlatBuffer.TableKey))
				fieldSetter(g, field, fieldConverter(fieldAccessor))
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
		g.Id("Finish"+def.Name+"Buffer").Call(
			Id("b"),
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
func generateStructMarshaler(f *File, def parser.Definition) {
	modelName := toModelName(def.Name)
	generateMarshalMessage(f, def, modelName)
	generateMarshal(f, def, modelName)
}
