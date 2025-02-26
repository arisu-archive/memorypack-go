package memorypack_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

// TestWriter tests the Writer class directly.
func TestWriter(t *testing.T) {
	t.Run("EnsureCapacity", func(t *testing.T) {
		// Start with a small buffer and write more than capacity
		writer := memorypack.NewWriter(2)

		// Write enough bytes to trigger capacity increase
		for i := 0; i < 100; i++ {
			writer.WriteByte(byte(i))
		}

		// Verify the bytes were written correctly
		bytes := writer.GetBytes()
		if len(bytes) != 100 {
			t.Errorf("Expected 100 bytes, got %d", len(bytes))
		}

		for i := 0; i < 100; i++ {
			if bytes[i] != byte(i) {
				t.Errorf("Expected byte %d at index %d, got %d", i, i, bytes[i])
			}
		}
	})
}

// TestReader tests the Reader class directly.
func TestReader(t *testing.T) {
	t.Run("ReadBeyondBuffer", func(t *testing.T) {
		// Create a small buffer
		reader := memorypack.NewReader([]byte{1, 2, 3})

		// Read valid data
		b1, err := reader.ReadByte()
		if err != nil || b1 != 1 {
			t.Errorf("Expected 1, got %d, err: %v", b1, err)
		}

		b2, err := reader.ReadByte()
		if err != nil || b2 != 2 {
			t.Errorf("Expected 2, got %d, err: %v", b2, err)
		}

		b3, err := reader.ReadByte()
		if err != nil || b3 != 3 {
			t.Errorf("Expected 3, got %d, err: %v", b3, err)
		}

		// Try to read beyond buffer
		_, err = reader.ReadByte()
		if err == nil {
			t.Error("Expected error when reading beyond buffer, got nil")
		}
	})
}

// TestCustomTypes tests serialization of custom structs with tags.
func TestCustomTypes(t *testing.T) {
	t.Run("StructWithTags", func(t *testing.T) {
		type TaggedStruct struct {
			First  string `memorypack:"2"` // Out of order on purpose
			Second int    `memorypack:"1"`
			Third  bool   `memorypack:"0"`
		}

		original := TaggedStruct{
			First:  "hello",
			Second: 42,
			Third:  true,
		}

		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		var result TaggedStruct
		if err = memorypack.Deserialize(data, &result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if result.First != original.First ||
			result.Second != original.Second ||
			result.Third != original.Third {
			t.Errorf("Result mismatch: got %+v, want %+v", result, original)
		}
	})

	t.Run("StructWithImmutableFields", func(t *testing.T) {
		type TestStructA struct {
			PublicField *string
		}

		type TestStructB struct {
			PublicField *string
			Immutable   []TestStructA
		}

		publicField := "public"
		privateField := "private"
		original := TestStructB{
			PublicField: &publicField,
			Immutable:   []TestStructA{{PublicField: &privateField}},
		}

		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		var result TestStructB
		if err = memorypack.Deserialize(data, &result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}
	})

	t.Run("SkippedFields", func(t *testing.T) {
		type SkipStruct struct {
			Include string
			Skip    string `memorypack:"-"`
			Another string
		}

		original := SkipStruct{
			Include: "visible",
			Skip:    "invisible",
			Another: "also visible",
		}

		data, err := memorypack.Serialize(&original)
		if err != nil {
			t.Fatalf("Serialize failed: %v", err)
		}

		var result SkipStruct
		if err = memorypack.Deserialize(data, &result); err != nil {
			t.Fatalf("Deserialize failed: %v", err)
		}

		if result.Include != original.Include || result.Another != original.Another {
			t.Errorf("Result mismatch for included fields: got %+v, want %+v", result, original)
		}

		if result.Skip != "" {
			t.Errorf("Skipped field should be empty, got: %s", result.Skip)
		}
	})
}

func BenchmarkSerialization(b *testing.B) {
	// Create a large slice (1M items)
	largeSlice := make([]int, 1000000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	b.ResetTimer()
	for range b.N {
		_, err := memorypack.Serialize(&largeSlice)
		if err != nil {
			b.Fatalf("Serialize failed: %v", err)
		}
	}

	b.SetBytes(int64(1000000 * 8)) // 8 bytes per int64
}

// BenchmarkDeserialization benchmarks the deserialization of large data structures.
func BenchmarkDeserialization(b *testing.B) {
	// Create and serialize a large slice first
	largeSlice := make([]int, 1000000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	data, err := memorypack.Serialize(&largeSlice)
	if err != nil {
		b.Fatalf("Serialize failed: %v", err)
	}

	b.ResetTimer()
	for range b.N {
		var result []int
		if err = memorypack.Deserialize(data, &result); err != nil {
			b.Fatalf("Deserialize failed: %v", err)
		}
		// Don't verify the result in benchmarks as it adds overhead
	}

	b.SetBytes(int64(len(data)))
}

// TestSpecialNumericCases tests edge cases with numeric values.
func TestSpecialNumericCases(t *testing.T) {
	t.Run("FloatSpecialValues", func(t *testing.T) {
		testRoundTrip(t, float32(math.NaN()))
		testRoundTrip(t, float32(math.Inf(1)))  // +Infinity
		testRoundTrip(t, float32(math.Inf(-1))) // -Infinity

		testRoundTrip(t, math.NaN())
		testRoundTrip(t, math.Inf(1))  // +Infinity
		testRoundTrip(t, math.Inf(-1)) // -Infinity
	})

	t.Run("IntegerBoundaries", func(t *testing.T) {
		// Test values around integer overflow boundaries
		testRoundTrip(t, int32(math.MaxInt32))
		testRoundTrip(t, int32(math.MaxInt32-1))
		testRoundTrip(t, int32(math.MinInt32))
		testRoundTrip(t, int32(math.MinInt32+1))

		testRoundTrip(t, int64(math.MaxInt64))
		testRoundTrip(t, int64(math.MaxInt64-1))
		testRoundTrip(t, int64(math.MinInt64))
		testRoundTrip(t, int64(math.MinInt64+1))
	})
}

// Helper function to test serialization and deserialization roundtrip.
func testRoundTrip[T any](t *testing.T, original T) {
	t.Helper()

	data, err := memorypack.Serialize(&original)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	var result T
	if err = memorypack.Deserialize(data, &result); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Special handling for NaN which doesn't equal itself
	if reflect.ValueOf(original).Kind() == reflect.Float32 ||
		reflect.ValueOf(original).Kind() == reflect.Float64 {
		originalFloat := reflect.ValueOf(original).Float()
		resultFloat := reflect.ValueOf(result).Float()

		if math.IsNaN(originalFloat) && !math.IsNaN(resultFloat) {
			t.Errorf("Expected NaN, got %v", resultFloat)
		} else if !math.IsNaN(originalFloat) && originalFloat != resultFloat {
			t.Errorf("Float mismatch: got %v, want %v", resultFloat, originalFloat)
		}
		return
	}

	// Normal comparison for other types
	if !reflect.DeepEqual(original, result) {
		t.Errorf("Result mismatch: got %+v, want %+v", result, original)
	}
}
