# Changelog

## [2.0.0](https://github.com/arisu-archive/memorypack-go/compare/v1.0.0...v2.0.0) (2026-09-12)


### ⚠ BREAKING CHANGES

* custom formatters now use MarshalMemoryPack and UnmarshalMemoryPack. Nullable pointer and UTF-8 string layouts now follow C# MemoryPack. Migrate incompatible old data by decoding with the prior implementation and re-encoding.

### Features

* restore core format compatibility ([#3](https://github.com/arisu-archive/memorypack-go/issues/3)) ([f9b7580](https://github.com/arisu-archive/memorypack-go/commit/f9b7580e38ea6bf055a1ec75c269d4b80210e8e1))
