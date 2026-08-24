package generator

import (
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/iancoleman/strcase"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func generateMarshalMessage(file *jen.File, definition parser.Definition, modelName string, withoutDecryption bool) {
	fieldSetter := func(group *jen.Group, field parser.Field, fieldAccessor jen.Code) {
		group.Id(definition.Name+"Add"+toFlatBuffersName(field.Name)).Call(jen.Id("b"), fieldAccessor)
	}

	file.Comment("MarshalModel marshals the struct into a FlatBuffers offset.")
	file.Func().Params(jen.Id("t").Op("*").Id(modelName)).Id("MarshalModel").Params(
		jen.Id("b").Op("*").Qual("github.com/google/flatbuffers/go", "Builder"),
	).Qual("github.com/google/flatbuffers/go", "UOffsetT").BlockFunc(func(group *jen.Group) {
		if !withoutDecryption {
			group.If(jen.Id("t").Dot("FlatBuffer").Dot("TableKey").Op("==").Nil()).Block(
				jen.Id("t").Dot("FlatBuffer").Dot("InitKey").Call(
					jen.Qual("github.com/arisu-archive/bluearchive-fbs-utils", "CreateTableKey").Call(
						jen.Lit(tableKeyName(definition.Name)),
					),
				),
			)
		}

		for _, field := range definition.Fields {
			fieldAccessor := jen.Id("t").Dot(toExportedName(field.Name))
			localName := toLocalName(field.Name)
			offsetVariable := localName + "Offset"
			offsetsVariable := localName + "Offsets"

			switch {
			case field.IsVector && field.IsString:
				fieldLength := jen.Len(fieldAccessor)
				group.Var().Id(offsetVariable).Qual("github.com/google/flatbuffers/go", "UOffsetT")
				group.Id(offsetsVariable).Op(":=").Make(
					jen.Index().Qual("github.com/google/flatbuffers/go", "UOffsetT"),
					fieldLength,
				)
				group.For(jen.Id("i").Op(":=").Range().Add(fieldAccessor)).BlockFunc(func(group *jen.Group) {
					value := fieldAccessor.Clone().Index(jen.Id("i"))
					if !withoutDecryption {
						value = encodeString(value)
					}
					group.Id(offsetsVariable).Index(jen.Id("i")).Op("=").Id("b").Dot("CreateString").Call(value)
				})
				group.Id(definition.Name+"Start"+toFlatBuffersName(field.Name)+"Vector").Call(jen.Id("b"), fieldLength)
				prependOffsets(group, offsetsVariable)
				group.Id(offsetVariable).Op("=").Id("b").Dot("EndVector").Call(fieldLength)

			case field.IsVector && field.IsNested():
				fieldLength := jen.Len(fieldAccessor)
				group.Var().Id(offsetVariable).Qual("github.com/google/flatbuffers/go", "UOffsetT")
				group.Id(offsetsVariable).Op(":=").Make(
					jen.Index().Qual("github.com/google/flatbuffers/go", "UOffsetT"),
					fieldLength,
				)
				group.For(jen.Id("i").Op(":=").Range().Add(fieldAccessor)).BlockFunc(func(group *jen.Group) {
					indexedField := fieldAccessor.Clone().Index(jen.Id("i"))
					if !withoutDecryption {
						group.Add(indexedField.Clone().Dot("InitKey").Call(jen.Id("t").Dot("FlatBuffer").Dot("TableKey")))
					}
					group.Id(offsetsVariable).Index(jen.Id("i")).Op("=").Add(
						indexedField.Clone().Dot("MarshalModel").Call(jen.Id("b")),
					)
				})
				group.Id(definition.Name+"Start"+toFlatBuffersName(field.Name)+"Vector").Call(jen.Id("b"), fieldLength)
				prependOffsets(group, offsetsVariable)
				group.Id(offsetVariable).Op("=").Id("b").Dot("EndVector").Call(fieldLength)

			case !field.IsVector && field.IsNested():
				if !withoutDecryption {
					group.Add(fieldAccessor.Clone().Dot("InitKey").Call(jen.Id("t").Dot("FlatBuffer").Dot("TableKey")))
				}
				group.Id(offsetVariable).Op(":=").Add(fieldAccessor.Clone().Dot("MarshalModel").Call(jen.Id("b")))

			case !field.IsVector && field.IsString:
				value := fieldAccessor
				if !withoutDecryption {
					value = encodeString(value)
				}
				group.Id(offsetVariable).Op(":=").Id("b").Dot("CreateString").Call(value)

			case field.IsVector:
				fieldLength := jen.Len(fieldAccessor)
				group.Id(definition.Name+"Start"+toFlatBuffersName(field.Name)+"Vector").Call(jen.Id("b"), fieldLength)
				group.For(
					jen.Id("i").Op(":=").Len(fieldAccessor).Op("-").Lit(1),
					jen.Id("i").Op(">=").Lit(0),
					jen.Id("i").Op("--"),
				).BlockFunc(func(group *jen.Group) {
					indexedField := fieldAccessor.Clone().Index(jen.Id("i"))
					goType := strcase.ToCamel(getBaseGoType(field).GoString())
					if field.IsEnum {
						goType = "Int32"
						indexedField = jen.Int32().Call(indexedField)
						if !withoutDecryption {
							indexedField = fieldConverter(indexedField)
						}
					} else if !withoutDecryption && field.IsPrimitive() && field.Type != "bool" {
						indexedField = fieldConverter(indexedField)
					}
					group.Id("b").Dot("Prepend" + goType).Call(indexedField)
				})
				group.Id(offsetVariable).Op(":=").Id("b").Dot("EndVector").Call(fieldLength)
			}
		}

		group.Id(definition.Name + "Start").Call(jen.Id("b"))
		for _, field := range definition.Fields {
			fieldAccessor := jen.Id("t").Dot(toExportedName(field.Name))
			offsetVariable := toLocalName(field.Name) + "Offset"

			switch {
			case field.IsVector:
				fieldSetter(group, field, jen.Id(offsetVariable))
			case field.IsNested() || field.IsString:
				fieldSetter(group, field, jen.Id(offsetVariable))
			case field.IsEnum:
				if !withoutDecryption {
					fieldAccessor = jen.Id(field.Type).Call(fieldConverter(jen.Int32().Call(fieldAccessor)))
				}
				fieldSetter(group, field, fieldAccessor)
			default:
				if !withoutDecryption && field.Type != "bool" {
					fieldAccessor = fieldConverter(fieldAccessor)
				}
				fieldSetter(group, field, fieldAccessor)
			}
		}
		group.Return(jen.Id(definition.Name + "End").Call(jen.Id("b")))
	})
	file.Line()
}

// encodeString generates a qualified call that invokes the Encode function in
// the bluearchive-fbs-utils package. Emitting a package-level helper instead
// would redeclare it when multiple schemas are generated into one package.
func encodeString(value jen.Code) *jen.Statement {
	return jen.Qual("github.com/arisu-archive/bluearchive-fbs-utils", "Encode").Call(
		value,
		jen.Id("t").Dot("FlatBuffer").Dot("TableKey"),
	)
}

func prependOffsets(group *jen.Group, offsetsVariable string) {
	group.For(
		jen.Id("i").Op(":=").Len(jen.Id(offsetsVariable)).Op("-").Lit(1),
		jen.Id("i").Op(">=").Lit(0),
		jen.Id("i").Op("--"),
	).Block(
		jen.Id("b").Dot("PrependUOffsetT").Call(jen.Id(offsetsVariable).Index(jen.Id("i"))),
	)
}

func generateMarshal(file *jen.File, modelName string) {
	file.Comment("Marshal marshals the struct into a FlatBuffers buffer.")
	file.Func().Params(jen.Id("t").Op("*").Id(modelName)).Id("Marshal").Params().Params(
		jen.Index().Byte(),
		jen.Error(),
	).Block(
		jen.Id("b").Op(":=").Qual("github.com/google/flatbuffers/go", "NewBuilder").Call(jen.Lit(0)),
		jen.Id("b").Dot("Finish").Call(jen.Id("t").Dot("MarshalModel").Call(jen.Id("b"))),
		jen.Return(jen.Id("b").Dot("FinishedBytes").Call(), jen.Nil()),
	)
	file.Line()
}

func generateTableMarshaler(file *jen.File, definition parser.Definition, withoutDecryption bool) {
	modelName := toModelName(definition.Name)
	generateMarshalMessage(file, definition, modelName, withoutDecryption)
	generateMarshal(file, modelName)
}

func tableKeyName(definitionName string) string {
	return strings.ReplaceAll(strings.ReplaceAll(definitionName, "ExcelTable", ""), "Excel", "")
}
