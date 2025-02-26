# MemoryPack-Go

MemoryPack is a high-performance binary serialization format designed for extreme optimization in C# and Unity, now available for Go. It focuses on speed, memory efficiency, and ease of use, making it ideal for game development, microservices, and performance-critical applications.

## Installation

```bash
go get github.com/arisu-archive/memorypack-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/yourusername/memorypack-go"
)

type Person struct {
    Name    string
    Age     int
    Address string
}

func main() {
    // Serialize
    person := Person{
        Name:    "John Doe",
        Age:     30,
        Address: "123 Main St",
    }
    
    data, err := memorypack.Marshal(&person)
    if err != nil {
        panic(err)
    }
    
    // Deserialize
    var newPerson Person
    err = memorypack.Unmarshal(data, &newPerson)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("%+v\n", newPerson)
}
```

## Supported Types

- Basic types: `int`, `float`, `bool`, `string`, `[]byte`
- Collections: `[]T`, `map[K]V`, `slice`, `array`
- Structs: `struct` with `memorypack` tags
- Pointers: `*T`
- Custom types: types that implement `Marshaler` and `Unmarshaler` interfaces

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
