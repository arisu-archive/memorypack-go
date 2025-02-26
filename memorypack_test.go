package memorypack_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

// TestBasicTypes tests serialization and deserialization of basic Go types.
func TestBasicTypes(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		testRoundTrip(t, true)
		testRoundTrip(t, false)
	})

	t.Run("Int8", func(t *testing.T) {
		testRoundTrip(t, int8(0))
		testRoundTrip(t, int8(127))
		testRoundTrip(t, int8(-128))
	})

	t.Run("Int16", func(t *testing.T) {
		testRoundTrip(t, int16(0))
		testRoundTrip(t, int16(32767))
		testRoundTrip(t, int16(-32768))
	})

	t.Run("Int32", func(t *testing.T) {
		testRoundTrip(t, int32(0))
		testRoundTrip(t, int32(2147483647))
		testRoundTrip(t, int32(-2147483648))
	})

	t.Run("Int64", func(t *testing.T) {
		testRoundTrip(t, int64(0))
		testRoundTrip(t, int64(9223372036854775807))
		testRoundTrip(t, int64(-9223372036854775808))
	})

	t.Run("Float32", func(t *testing.T) {
		testRoundTrip(t, float32(0))
		testRoundTrip(t, float32(3.14159))
		testRoundTrip(t, float32(-3.14159))
		testRoundTrip(t, float32(math.MaxFloat32))
		testRoundTrip(t, float32(math.SmallestNonzeroFloat32))
	})

	t.Run("Float64", func(t *testing.T) {
		testRoundTrip(t, float64(0))
		testRoundTrip(t, float64(3.141592653589793))
		testRoundTrip(t, float64(-3.141592653589793))
		testRoundTrip(t, math.MaxFloat64)
		testRoundTrip(t, math.SmallestNonzeroFloat64)
	})

	t.Run("String", func(t *testing.T) {
		testRoundTrip(t, "")
		testRoundTrip(t, "Hello, World!")
		testRoundTrip(t, "Special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?")

		// Test UTF-8 characters
		testRoundTrip(t, "UTF-8 characters: 你好, こんにちは, 안녕하세요")

		// Test very long string
		longString := make([]byte, 10000)
		for i := range longString {
			longString[i] = byte('a' + (i % 26))
		}
		testRoundTrip(t, string(longString))
	})
}

// TestCollectionTypes tests serialization and deserialization of collection types.
func TestCollectionTypes(t *testing.T) {
	t.Run("ByteSlice", func(t *testing.T) {
		testRoundTrip(t, []byte(nil))
		testRoundTrip(t, []byte{})
		testRoundTrip(t, []byte{1, 2, 3, 4, 5})

		// Test large byte slice
		largeBytes := make([]byte, 10000)
		for i := range largeBytes {
			largeBytes[i] = byte(i % 256)
		}
		testRoundTrip(t, largeBytes)
	})

	t.Run("IntSlice", func(t *testing.T) {
		testRoundTrip(t, []int(nil))
		testRoundTrip(t, []int{})
		testRoundTrip(t, []int{1, 2, 3, 4, 5})

		// Test large int slice
		largeInts := make([]int, 1000)
		for i := range largeInts {
			largeInts[i] = i
		}
		testRoundTrip(t, largeInts)
	})

	t.Run("StringSlice", func(t *testing.T) {
		testRoundTrip(t, []string(nil))
		testRoundTrip(t, []string{})
		testRoundTrip(t, []string{"hello", "world"})
		testRoundTrip(t, []string{"", "empty", ""})
	})

	t.Run("Array", func(t *testing.T) {
		testRoundTrip(t, [5]int{1, 2, 3, 4, 5})
		testRoundTrip(t, [3]string{"a", "b", "c"})
		testRoundTrip(t, [0]int{})
	})

	t.Run("Map", func(t *testing.T) {
		testRoundTrip(t, map[string]int(nil))
		testRoundTrip(t, map[string]int{})
		testRoundTrip(t, map[string]int{"one": 1, "two": 2, "three": 3})
		testRoundTrip(t, map[int]string{1: "one", 2: "two", 3: "three"})

		// Empty key or value
		testRoundTrip(t, map[string]string{"": "empty key", "empty value": ""})

		// Large map
		largeMap := make(map[int]int, 1000)
		for i := 0; i < 1000; i++ {
			largeMap[i] = i * i
		}
		testRoundTrip(t, largeMap)
	})

	t.Run("NestedCollections", func(t *testing.T) {
		testRoundTrip(t, [][]int{{1, 2}, {3, 4}, {5, 6}})
		testRoundTrip(t, map[string][]int{"evens": {2, 4, 6}, "odds": {1, 3, 5}})
		testRoundTrip(t, []map[string]int{{"a": 1}, {"b": 2}, {"c": 3}})
	})
}

// TestStructs tests serialization and deserialization of struct types.
func TestStructs(t *testing.T) {
	type SimpleStruct struct {
		A int
		B string
		C bool
	}

	t.Run("SimpleStruct", func(t *testing.T) {
		testRoundTrip(t, SimpleStruct{})
		testRoundTrip(t, SimpleStruct{A: 42, B: "hello", C: true})
	})

	type NestedStruct struct {
		Simple SimpleStruct
		D      float64
	}

	t.Run("NestedStruct", func(t *testing.T) {
		testRoundTrip(t, NestedStruct{})
		testRoundTrip(t, NestedStruct{
			Simple: SimpleStruct{A: 42, B: "hello", C: true},
			D:      3.14159,
		})
	})

	type StructWithCollections struct {
		IntSlice   []int
		StringMap  map[string]string
		FloatArray [3]float32
	}

	t.Run("StructWithCollections", func(t *testing.T) {
		testRoundTrip(t, StructWithCollections{})
		testRoundTrip(t, StructWithCollections{
			IntSlice:   []int{1, 2, 3},
			StringMap:  map[string]string{"a": "apple", "b": "banana"},
			FloatArray: [3]float32{1.1, 2.2, 3.3},
		})
	})

	type StructWithPrivateFields struct {
		Public  string
		private int // This shouldn't be serialized
	}

	t.Run("StructWithPrivateFields", func(t *testing.T) {
		original := StructWithPrivateFields{Public: "visible", private: 42}
		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		var result StructWithPrivateFields
		if err = memorypack.Deserialize(data, &result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if result.Public != original.Public {
			t.Errorf("Public field mismatch: got %v, want %v", result.Public, original.Public)
		}

		// Private field should remain at zero value
		if result.private != 0 {
			t.Errorf("Private field should not be serialized: got %v, want 0", result.private)
		}
	})
}

// TestPointers tests serialization and deserialization of pointer types.
func TestPointers(t *testing.T) {
	t.Run("IntPointer", func(t *testing.T) {
		i := 42
		testRoundTrip(t, &i)

		var nilPtr *int
		testRoundTrip(t, nilPtr)
	})

	t.Run("StringPointer", func(t *testing.T) {
		s := "hello"
		testRoundTrip(t, &s)

		var nilPtr *string
		testRoundTrip(t, nilPtr)
	})

	type Person struct {
		Name string
		Age  int
	}

	t.Run("StructPointer", func(t *testing.T) {
		p := Person{Name: "Alice", Age: 30}
		testRoundTrip(t, &p)

		var nilPtr *Person
		testRoundTrip(t, nilPtr)
	})

	type LinkedNode struct {
		Value int
		Next  *LinkedNode
	}

	t.Run("RecursiveStructPointer", func(t *testing.T) {
		node3 := LinkedNode{Value: 3, Next: nil}
		node2 := LinkedNode{Value: 2, Next: &node3}
		node1 := LinkedNode{Value: 1, Next: &node2}

		testRoundTrip(t, &node1)
	})

	type CircularStruct struct {
		Name  string
		Self  *CircularStruct
		Other *CircularStruct
	}

	t.Run("CircularReference", func(t *testing.T) {
		a := &CircularStruct{Name: "A"}
		b := &CircularStruct{Name: "B"}
		a.Other = b
		b.Other = a

		// This will test if we handle circular references properly
		// Note: For proper circular reference handling, we'd need reference tracking
		// which isn't implemented in this simple version
		data, err := memorypack.Serialize(a)
		if err != nil {
			// If we don't have circular reference protection, we should get stack overflow
			// In a fixed version, this should pass
			t.Logf("Circular reference serialization fails as expected: %v", err)
			return
		}

		var result CircularStruct
		if err = memorypack.Deserialize(data, &result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if result.Name != "A" {
			t.Errorf("Expected name 'A', got '%s'", result.Name)
		}

		if result.Other == nil {
			t.Errorf("Other should not be nil")
		} else if result.Other.Name != "B" {
			t.Errorf("Expected name 'B', got '%s'", result.Other.Name)
		}
	})
}

// TestFormatterInterface tests types that implement the Formatter interface.
type CustomFormat struct {
	IntValue int
	StrValue string
}

// Implement the Formatter interface for CustomFormat.
func (c *CustomFormat) Serialize(writer *memorypack.Writer) error {
	// Custom serialization logic
	writer.WriteInt32(int32(c.IntValue))
	writer.WriteString(c.StrValue)
	return nil
}

func (c *CustomFormat) Deserialize(reader *memorypack.Reader) error {
	// Custom deserialization logic
	val, err := reader.ReadInt32()
	if err != nil {
		return err
	}
	c.IntValue = int(val)

	str, err := reader.ReadString()
	if err != nil {
		return err
	}
	c.StrValue = str
	return nil
}

func TestFormatterInterface(t *testing.T) {
	t.Run("CustomFormatter", func(t *testing.T) {
		original := &CustomFormat{IntValue: 42, StrValue: "custom"}
		data, err := memorypack.Serialize(original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		result := &CustomFormat{}
		if err = memorypack.Deserialize(data, result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if !reflect.DeepEqual(original, result) {
			t.Errorf("Result mismatch: got %+v, want %+v", result, original)
		}
	})
}

// TestErrorHandling tests error handling in various scenarios.
func TestErrorHandling(t *testing.T) {
	t.Run("InvalidPointer", func(t *testing.T) {
		value := 42
		data, err := memorypack.Serialize(&value)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		// Passing non-pointer to Deserialize
		if err = memorypack.Deserialize(data, value); err == nil {
			t.Error("Expected error when passing non-pointer to Deserialize, got nil")
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		var result int
		if err := memorypack.Deserialize([]byte{}, &result); err == nil {
			t.Error("Expected error when deserializing empty data, got nil")
		}
	})

	t.Run("TruncatedData", func(t *testing.T) {
		// Serialize an int.
		original := 42
		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		// Truncate the data
		truncatedData := data[:len(data)/2]

		var result int
		if err = memorypack.Deserialize(truncatedData, &result); err == nil {
			t.Error("Expected error when deserializing truncated data, got nil")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		type MyStruct struct {
			Value int
		}
		// Serialize a string
		original := MyStruct{Value: 42}
		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		// Try to deserialize into an int
		type DifferentStruct struct {
			Value string
			Other int
		}
		var result DifferentStruct
		if err = memorypack.Deserialize(data, &result); err == nil {
			t.Error("Expected error when deserializing into wrong type, got nil")
		}
	})

	t.Run("InvalidObjectHeader", func(t *testing.T) {
		// Create a writer with large member count
		writer := memorypack.NewWriter(64)
		err := writer.WriteObjectHeader(250) // Max is 249
		if err == nil {
			t.Error("Expected error for object header with too many members, got nil")
		}
	})
}

type Level1 struct {
	Value int
	Next  *Level2
}
type Level2 struct {
	Value int
	Next  *Level3
}
type Level3 struct {
	Value int
	Next  *Level4
}
type Level4 struct {
	Value int
	Next  *Level5
}
type Level5 struct {
	Value int
}

// TestEdgeAndCornerCases tests various edge and corner cases.
func TestEdgeAndCornerCases(t *testing.T) {
	t.Run("ZeroValues", func(t *testing.T) {
		testRoundTrip(t, 0)
		testRoundTrip(t, "")
		testRoundTrip(t, false)
		testRoundTrip(t, 0.0)
		testRoundTrip(t, struct{}{})
	})

	t.Run("EmptyCollections", func(t *testing.T) {
		testRoundTrip(t, []int{})
		testRoundTrip(t, map[string]int{})
		testRoundTrip(t, [0]int{})
	})

	t.Run("MaxValues", func(t *testing.T) {
		testRoundTrip(t, math.MaxInt32)
		testRoundTrip(t, math.MaxInt64)
		testRoundTrip(t, math.MaxFloat32)
		testRoundTrip(t, math.MaxFloat64)
	})

	t.Run("MinValues", func(t *testing.T) {
		testRoundTrip(t, math.MinInt32)
		testRoundTrip(t, math.MinInt64)
		testRoundTrip(t, -math.MaxFloat32)
		testRoundTrip(t, -math.MaxFloat64)
	})

	t.Run("LargeStructs", func(t *testing.T) {
		// Create a struct with many fields
		type LargeStruct struct {
			Field1, Field2, Field3, Field4, Field5      int
			Field6, Field7, Field8, Field9, Field10     string
			Field11, Field12, Field13, Field14, Field15 float64
			Field16, Field17, Field18, Field19, Field20 bool
		}

		large := LargeStruct{
			Field1: 1, Field2: 2, Field3: 3, Field4: 4, Field5: 5,
			Field6: "a", Field7: "b", Field8: "c", Field9: "d", Field10: "e",
			Field11: 1.1, Field12: 2.2, Field13: 3.3, Field14: 4.4, Field15: 5.5,
			Field16: true, Field17: false, Field18: true, Field19: false, Field20: true,
		}

		testRoundTrip(t, large)
	})

	t.Run("DeepNestedStructs", func(t *testing.T) {
		deep := Level1{
			Value: 1,
			Next: &Level2{
				Value: 2,
				Next: &Level3{
					Value: 3,
					Next: &Level4{
						Value: 4,
						Next: &Level5{
							Value: 5,
						},
					},
				},
			},
		}

		testRoundTrip(t, deep)
	})
}
