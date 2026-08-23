# BlueArchive FlatBuffers Generator

[![Go Reference](https://pkg.go.dev/badge/github.com/arisu-archive/bluearchive-fbs-generator.svg)](https://pkg.go.dev/github.com/arisu-archive/bluearchive-fbs-generator)
[![CI](https://github.com/arisu-archive/bluearchive-fbs-generator/actions/workflows/ci.yml/badge.svg)](https://github.com/arisu-archive/bluearchive-fbs-generator/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Generates Go DTOs that marshal and unmarshal Blue Archive FlatBuffers tables,
including the field-conversion and nested table-key behavior used by game data.

## Contents

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [Supported schemas](#supported-schemas)
- [Testing](#testing)
- [License](#license)

## Features

- Parses FlatBuffers schemas and local includes.
- Generates formatted Go DTO source without requiring a filesystem write.
- Marshals and unmarshals scalar, string, vector, enum, and nested table fields.
- Preserves explicit root keys and propagates root keys through nested DTO trees.
- Supports raw generation with all conversion logic disabled.

## Install

```bash
go install github.com/arisu-archive/bluearchive-fbs-generator@latest
```

## Usage

```bash
bluearchive-fbs-generator \
  -input ./schemas \
  -output ./flatdata \
  -package flatdata
```

Short forms (`-i`, `-o`, and `-p`) are also accepted.

| Flag | Default | Description |
|---|---:|---|
| `-i`, `-input` | required | Directory containing `.fbs` schemas. |
| `-o`, `-output` | `.` | Directory for generated Go files. |
| `-p`, `-package` | `model` | Package name for generated files. |
| `-without-decryption` | `false` | Skip field encryption and decryption conversions. |

The generator can also render source in memory for library callers:

```go
schema, err := parser.ParseFile(ctx, "schema.fbs")
if err != nil {
    return err
}
files, err := generator.Render(schema, generator.Options{PackageName: "flatdata"})
```

## Supported schemas

Tables, enums, strings, supported scalar fields, vectors, and nested tables are
supported. Structs and unions are rejected explicitly until their different
FlatBuffers layouts are implemented. With conversion enabled, `byte`, `short`,
and `ushort` fields are rejected because the current conversion library does not
support those Go scalar types.

## Testing

Install `flatc` and Python 3, then run:

```bash
go test -race ./...
go vet ./...
golangci-lint run ./...
python testdata/verify_table_key_vectors.py
```

The generator tests compile and execute a temporary package containing both
`flatc` bindings and generated DTOs.

## License

Licensed under the [MIT License](LICENSE).
