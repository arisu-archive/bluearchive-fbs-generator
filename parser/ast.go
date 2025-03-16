package parser

// SchemaType represents the type of schema element
type SchemaType int

const (
	TypeStruct SchemaType = iota
	TypeTable
	TypeEnum
	TypeUnion
)

// Field represents a field in a struct or table
type Field struct {
	Name     string
	Type     string
	ID       int
	IsVector bool
	IsString bool
	IsStruct bool
	IsTable  bool
	IsEnum   bool
	IsUnion  bool
}

func (f Field) IsNested() bool {
	return (f.IsTable || f.IsStruct || f.IsUnion)
}

func (f Field) IsNestedVector() bool {
	return f.IsNested() && f.IsVector
}

func (f Field) IsPrimitive() bool {
	return f.Type == "bool" || f.Type == "byte" || f.Type == "short" || f.Type == "ushort" ||
		f.Type == "int" || f.Type == "uint" || f.Type == "long" || f.Type == "ulong" ||
		f.Type == "float" || f.Type == "double" || f.Type == "string"
}

// Definition represents a struct, table, enum, or union definition
type Definition struct {
	Name       string
	Type       SchemaType
	Fields     []Field
	EnumValues map[string]int // Only used for enums
}

// Schema represents a complete FlatBuffers schema
type Schema struct {
	Namespace   string
	FileName    string
	Includes    []string
	Definitions []Definition
	RootTable   string
}
