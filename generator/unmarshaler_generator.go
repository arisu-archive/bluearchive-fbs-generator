package generator

import (
	"strings"

	. "github.com/dave/jennifer/jen"

	"github.com/arisu-archive/bluearchive-fbs-generator/parser"
)

// generateUnmarshalMessage is a helper function that generates code for the UnmarshalMessage method.
// Function Signature:
// func (t *<modelName>) Unmarshal(data []byte) error
func generateUnmarshalMessage(f *File, def parser.Definition, modelName string, withoutDecryption bool) {
	// Define helper function to handle type conversion
	handleFieldType := func(tableField *Statement, field parser.Field, val Code) {
		switch {
		case field.IsEnum:
			tableField.Op("=").Add(Id(field.Type).Call(fieldConverter(Id("int32").Call(val))))
		case field.IsString:
			tableField.Op("=").Add(fieldConverter(Id("string").Call(val)))
		case field.IsPrimitive() && field.Type == "bool":
			tableField.Op("=").Add(val)
		default:
			tableField.Op("=").Add(fieldConverter(val))
		}
	}

	f.Comment("UnmarshalMessage unmarshals the struct from a FlatBuffers buffer")
	f.Func().Params(Id("t").Op("*").Id(modelName)).Id("UnmarshalMessage").Params(
		Id("e").Op("*").Id(def.Name),
	).Params(
		Error(),
	).BlockFunc(func(g *Group) {
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

		for _, field := range def.Fields {
			tableField := g.Id("t").Dot(toExportedName(field.Name))
			excelField := Id("e").Dot(toExportedName(field.Name))
			if field.IsVector {
				excelFieldLength := Id("e").Dot(toExportedName(field.Name) + "Length").Call()
				// t.<fieldName> := make([]<fieldType>, excelFieldLength)
				tableField.Op("=").Make(getGoType(field), excelFieldLength)
				// for i := range excelFieldLength
				g.For(Id("i").Op(":=").Range().Add(excelFieldLength)).BlockFunc(func(g *Group) {
					if field.IsNestedVector() {
						// d := new(<fieldType>)
						g.Id("d").Op(":=").New(Id(field.Type))
						// if !e.<fieldName>(d, i) {
						g.If(Op("!").Add(excelField).Call(Id("d"), Id("i"))).Block(
							Return(Qual("errors", "New").Call(Lit("failed to unmarshal data"))),
						)
						// t.<fieldName>[i].UnmarshalMessage(d)
						g.Id("t").Dot(toExportedName(field.Name)).Index(Id("i")).Dot("UnmarshalMessage").Call(Id("d"))
						return
					}
					// t.<fieldName>[i] = excelField[i]
					excelFieldValue := excelField.Call(Id("i"))
					if field.IsString {
						excelFieldValue = fieldConverter(Id("string").Call(excelFieldValue))
					} else if field.IsEnum {
						excelFieldValue = Id(field.Type).Call(fieldConverter(Id("int32").Call(excelFieldValue)))
					} else if field.IsPrimitive() && field.Type != "bool" {
						// bool is a special case, it doesn't need to be converted
						excelFieldValue = fieldConverter(excelFieldValue)
					}
					g.Id("t").Dot(toExportedName(field.Name)).Index(Id("i")).Op("=").Add(excelFieldValue)
				})
			} else if field.IsNested() {
				// t.<fieldName> = excelField(st)
				tableField.Dot("UnmarshalMessage").Call(excelField.Call(Nil()))
			} else {
				// t.<fieldName> = excelField()
				handleFieldType(tableField, field, excelField.Call())
			}
		}
		g.Return(Nil())
	})

	f.Line()
}

// generateUnmarshal is a helper function that generates code for the UnmarshalMessage method.
// Function Signature:
// func (t *<modelName>) UnmarshalMessage(e *<def.Name>) error
func generateUnmarshal(f *File, def parser.Definition, modelName string) {
	f.Comment("Unmarshal unmarshals the struct from a FlatBuffers buffer")
	f.Func().Params(Id("t").Op("*").Id(modelName)).Id("Unmarshal").Params(
		Id("data").Index().Add(Byte()),
	).Params(
		Error(),
	).BlockFunc(func(g *Group) {
		// root := GetRootAs<modelName>(data, 0)
		g.Id("root").Op(":=").Id("GetRootAs"+def.Name).Call(Id("data"), Lit(0))
		// err := t.UnmarshalMessage(root)
		g.Id("err").Op(":=").Id("t").Dot("UnmarshalMessage").Call(Id("root"))
		// If err != nil, return err
		g.If(Id("err").Op("!=").Nil()).Block(
			Return(Id("err")),
		)
		// return nil
		g.Return(Nil())
	})

	f.Line()
}

// generateStructUnmarshaler generates a Go unmarshaler for a FlatBuffers struct.
func generateStructUnmarshaler(f *File, def parser.Definition, withoutDecryption bool) {
	modelName := toModelName(def.Name)
	generateUnmarshalMessage(f, def, modelName, withoutDecryption)
	generateUnmarshal(f, def, modelName)
}
