package main

import (
	"fmt"
	"log"

	"github.com/arisu-archive/memorypack-go"
)

// Person demonstrates MemoryPack serialization.
type Person struct {
	ID        int64
	Name      string
	Age       int32
	IsActive  bool
	Tags      []string
	Addresses map[string]string
	Parent    *Person // Nested reference
}

func PrintHex(data []byte) {
	for i, b := range data {
		if i > 0 && i%16 == 0 {
			fmt.Println()
		}
		fmt.Printf("%02X ", b)
	}
}

func main() {
	// Create a person
	p := &Person{
		ID:       12345,
		Name:     "John Doe",
		Age:      42,
		IsActive: true,
		Tags:     []string{"programmer", "gopher"},
		Addresses: map[string]string{
			"home": "123 Main St",
			"work": "456 Office Blvd",
		},
		Parent: &Person{
			ID:       6789,
			Name:     "Jane Doe",
			Age:      65,
			IsActive: true,
		},
	}

	// Serialize
	data, err := memorypack.Serialize(p)
	if err != nil {
		log.Fatalf("Serialization error: %v", err)
	}
	fmt.Printf("Serialized to %d bytes\n", len(data))

	// Print the serialized data as a hex string with 2 bytes per line
	PrintHex(data)
	fmt.Println()

	// Deserialize
	newPerson := &Person{}
	if err := memorypack.Deserialize(data, newPerson); err != nil {
		log.Fatalf("Deserialization error: %v", err)
	}

	// Verify
	fmt.Printf("Person: %+v\n", newPerson)
	fmt.Printf("Deserialized: %s, %d years old\n", newPerson.Name, newPerson.Age)
	fmt.Printf("Tags: %v\n", newPerson.Tags)
	fmt.Printf("Home address: %s\n", newPerson.Addresses["home"])
	// Create a person

	i := 12345
	// Serialize
	data, err = memorypack.Serialize(&i)
	if err != nil {
		log.Fatalf("Serialization error: %v", err)
	}
	fmt.Printf("Serialized to %d bytes\n", len(data))

	// Print the serialized data as a hex string with 2 bytes per line
	PrintHex(data)
	fmt.Println()
	var v *int32
	if err := memorypack.Deserialize(data, &v); err != nil {
		log.Fatalf("Deserialization error: %v", err)
	}
	fmt.Printf("Deserialized: %d\n", *v)

	var nilPerson *Person
	data, err = memorypack.Serialize(nilPerson)
	if err != nil {
		log.Fatalf("Serialization error: %v", err)
	}
	PrintHex(data)
	fmt.Println()
	if err := memorypack.Deserialize(data, &nilPerson); err != nil {
		log.Fatalf("Deserialization error: %v", err)
	}
	fmt.Printf("Deserialized: %+v\n", nilPerson)
}
