# MemoryPack-Go

[![Go Reference](https://pkg.go.dev/badge/github.com/arisu-archive/memorypack-go.svg)](https://pkg.go.dev/github.com/arisu-archive/memorypack-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go implementation of the [MemoryPack binary format](https://github.com/Cysharp/MemoryPack#binary-wire-format-specification), with C# compatibility fixtures. Both applications must share the same schema.

[Features](#features) · [Installation](#installation) · [Quick start](#quick-start) · [Compatibility](#compatibility-and-migration) · [Testing](#testing) · [License](#license)

## Features

- Signed and unsigned integers, floating-point values, booleans, strings, and byte slices.
- Structs with explicit field order and ignored fields; slices, arrays, and maps.
- Interface unions with explicit tags, including nested values and wide tags.
- Nullable scalars using `Nullable[T]` or scalar pointer fields.
- UTF-8 and UTF-16 strings, including null strings through `*string`.
- Custom `MarshalMemoryPack` / `UnmarshalMemoryPack` methods at every nesting level.
- Ordinary schema evolution and opt-in `VersionTolerant` objects.
- Bounded decoding with errors for malformed headers, lengths, and excessive depth.

See the [package documentation](https://pkg.go.dev/github.com/arisu-archive/memorypack-go) for type mappings, formatter contracts, reader limits, and buffer ownership.

## Installation

Requires Go 1.22.5 or later.

```sh
go get github.com/arisu-archive/memorypack-go
```

## Quick start

Import `github.com/arisu-archive/memorypack-go` as `memorypack`. Define the interface and its concrete types:

```go
type Event interface{ isEvent() }

type Created struct{ ID int32 }

func (*Created) isEvent() {}
```

Register the union schema once, then pass the interface by address to preserve its declared type:

```go
err := memorypack.RegisterUnion[Event](
    memorypack.UnionMember{Tag: 0, Value: (*Created)(nil)},
)
if err != nil {
    panic(err)
}

var event Event = &Created{ID: 42}
data, err := memorypack.Serialize(&event)
if err != nil {
    panic(err)
}

var decoded Event
if err := memorypack.Deserialize(data, &decoded); err != nil {
    panic(err)
}
// decoded contains *Created{ID: 42}.
```

## Compatibility and migration

Go `int` and `uint` use 64-bit wire values. Use `int32` for C# `int` and `uint32` for C# `uint`. Go structs use the MemoryPack object format; arbitrary C# unmanaged struct layouts are not inferred.

`Serialize(&value)` treats the outer pointer as an address. For a scalar pointer that represents a nullable value, pass its address too, or use `Nullable[T]` explicitly. `Serialize(interfaceValue)` encodes the concrete type; `Serialize(&interfaceValue)` uses the registered union.

Ordinary objects can read older data with fewer trailing fields. Structs implementing `MemoryPackVersionTolerant()` use field lengths to support additions and removals. Each serialized field needs a permanent numeric `memorypack:"0"` order; do not reuse an old order for a different wire type. Missing fields retain their values when decoding into an existing object.

This restoration changes several previously incorrect contracts:

- Custom formatters now implement `MarshalMemoryPack(*Writer) error` and `UnmarshalMemoryPack(*Reader) error`, replacing the old method names.
- Scalar pointer fields now use C# nullable flags and padding. Null strings and collection pointers use four-byte null headers.
- UTF-8 string headers now count UTF-16 code units correctly.
- Custom formatters write their payload once and are honored inside objects and collections.

Data written with the old pointer layouts or incorrect custom formats needs the old decoder and an explicit re-encoding migration. There is no reliable automatic detection because MemoryPack is not self-describing.

Compression, circular reference tracking, streaming, .NET framework integrations, and arbitrary .NET-specific types remain outside this implementation. Map entry order is unspecified. Unpaired UTF-16 surrogates are rejected.

## Testing

```sh
go test ./...
go test -race ./...
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 30s .
```

Go tests consume checked-in C# fixtures without requiring .NET. The fixture generator pins MemoryPack 1.21.4 and locks its dependencies. With the .NET 9 SDK installed, reproduce and check its output:

```sh
dotnet restore testdata/csharp/Fixtures.csproj --locked-mode
dotnet run --project testdata/csharp/Fixtures.csproj --no-restore -- --check testdata/compatibility.json
```

Use `--write` instead of `--check` only when intentionally regenerating fixtures, then review the byte changes against the C# schemas.

## License

[MIT](LICENSE).
