package generator

import (
	"github.com/dave/jennifer/jen"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

func generateUnmarshalMessage(file *jen.File, definition parser.Definition, modelName string, withoutDecryption bool) {
	handleFieldType := func(tableField *jen.Statement, field parser.Field, value jen.Code) {
		if withoutDecryption {
			if field.IsString {
				tableField.Op("=").Add(jen.String().Call(value))
				return
			}
			tableField.Op("=").Add(value)
			return
		}
		switch {
		case field.IsEnum:
			tableField.Op("=").Add(jen.Id(field.Type).Call(fieldConverter(jen.Int32().Call(value))))
		case field.IsString:
			tableField.Op("=").Add(fieldConverter(jen.String().Call(value)))
		case field.IsPrimitive() && field.Type == "bool":
			tableField.Op("=").Add(value)
		default:
			tableField.Op("=").Add(fieldConverter(value))
		}
	}

	file.Comment("UnmarshalMessage unmarshals the struct from a FlatBuffers table.")
	file.Func().Params(jen.Id("t").Op("*").Id(modelName)).Id("UnmarshalMessage").Params(
		jen.Id("e").Op("*").Id(definition.Name),
	).Error().BlockFunc(func(group *jen.Group) {
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
			flatBufferField := jen.Id("e").Dot(toFlatBuffersName(field.Name))
			localName := toLocalName(field.Name)
			if field.IsVector {
				tableField := group.Id("t").Dot(toExportedName(field.Name))
				fieldLength := jen.Id("e").Dot(toFlatBuffersName(field.Name) + "Length").Call()
				tableField.Op("=").Make(getGoType(field), fieldLength)
				group.For(
					jen.Id("i").Op(":=").Lit(0),
					jen.Id("i").Op("<").Add(fieldLength),
					jen.Id("i").Op("++"),
				).BlockFunc(func(group *jen.Group) {
					if field.IsNestedVector() {
						generateNestedVectorUnmarshal(group, field, flatBufferField, withoutDecryption)
						return
					}

					fieldValue := flatBufferField.Call(jen.Id("i"))
					switch {
					case field.IsString:
						fieldValue = jen.String().Call(fieldValue)
						if !withoutDecryption {
							fieldValue = fieldConverter(fieldValue)
						}
					case field.IsEnum:
						if !withoutDecryption {
							fieldValue = jen.Id(field.Type).Call(fieldConverter(jen.Int32().Call(fieldValue)))
						}
					case !withoutDecryption && field.IsPrimitive() && field.Type != "bool":
						fieldValue = fieldConverter(fieldValue)
					}
					group.Id("t").Dot(toExportedName(field.Name)).Index(jen.Id("i")).Op("=").Add(fieldValue)
				})
				continue
			}

			if field.IsNested() {
				group.Id("t").Dot(toExportedName(field.Name)).Op("=").Add(getBaseGoType(field).Values())
				group.Id(localName).Op(":=").Add(flatBufferField.Call(jen.Nil()))
				group.If(jen.Id(localName).Op("!=").Nil()).BlockFunc(func(group *jen.Group) {
					if !withoutDecryption {
						group.Id("t").Dot(toExportedName(field.Name)).Dot("InitKey").Call(
							jen.Id("t").Dot("FlatBuffer").Dot("TableKey"),
						)
					}
					group.If(
						jen.Err().Op(":=").Id("t").Dot(toExportedName(field.Name)).Dot("UnmarshalMessage").Call(jen.Id(localName)),
						jen.Err().Op("!=").Nil(),
					).Block(
						jen.Return(jen.Qual("fmt", "Errorf").Call(
							jen.Lit("unmarshal "+field.Name+": %w"),
							jen.Err(),
						)),
					)
				})
				continue
			}

			tableField := group.Id("t").Dot(toExportedName(field.Name))
			handleFieldType(tableField, field, flatBufferField.Call())
		}
		group.Return(jen.Nil())
	})
	file.Line()
}

func generateNestedVectorUnmarshal(
	group *jen.Group,
	field parser.Field,
	flatBufferField *jen.Statement,
	withoutDecryption bool,
) {
	group.Id("child").Op(":=").New(jen.Id(field.Type))
	group.If(jen.Op("!").Add(flatBufferField).Call(jen.Id("child"), jen.Id("i"))).Block(
		jen.Return(jen.Qual("fmt", "Errorf").Call(
			jen.Lit("read "+field.Name+"[%d]"),
			jen.Id("i"),
		)),
	)
	if !withoutDecryption {
		group.Id("t").Dot(toExportedName(field.Name)).Index(jen.Id("i")).Dot("InitKey").Call(
			jen.Id("t").Dot("FlatBuffer").Dot("TableKey"),
		)
	}
	group.If(
		jen.Err().Op(":=").Id("t").Dot(toExportedName(field.Name)).Index(jen.Id("i")).Dot("UnmarshalMessage").Call(jen.Id("child")),
		jen.Err().Op("!=").Nil(),
	).Block(
		jen.Return(jen.Qual("fmt", "Errorf").Call(
			jen.Lit("unmarshal "+field.Name+"[%d]: %w"),
			jen.Id("i"),
			jen.Err(),
		)),
	)
}

func generateUnmarshal(file *jen.File, definition parser.Definition, modelName string) {
	file.Comment("Unmarshal unmarshals the struct from a FlatBuffers buffer.")
	file.Func().Params(jen.Id("t").Op("*").Id(modelName)).Id("Unmarshal").Params(
		jen.Id("data").Index().Byte(),
	).Error().Block(
		jen.Id("root").Op(":=").Id("GetRootAs"+definition.Name).Call(jen.Id("data"), jen.Lit(0)),
		jen.Return(jen.Id("t").Dot("UnmarshalMessage").Call(jen.Id("root"))),
	)
	file.Line()
}

func generateTableUnmarshaler(file *jen.File, definition parser.Definition, withoutDecryption bool) {
	modelName := toModelName(definition.Name)
	generateUnmarshalMessage(file, definition, modelName, withoutDecryption)
	generateUnmarshal(file, definition, modelName)
}
