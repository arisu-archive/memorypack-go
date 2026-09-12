package memorypack_test

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

func TestStringEncodingsAndFraming(t *testing.T) {
	value := "A界\U0001D11E"
	data, err := memorypack.SerializeWithOptions(value, memorypack.Options{StringEncoding: memorypack.UTF16})
	if err != nil || !bytes.Equal(data, csharpFixture(t, "utf16")) {
		t.Fatalf("UTF-16 encode: %X, error %v", data, err)
	}
	if _, err = memorypack.SerializeWithOptions(value, memorypack.Options{StringEncoding: 99}); err == nil {
		t.Fatal("accepted invalid encoding option")
	}
	r := memorypack.NewReader([]byte{255, 255, 255, 255, 42})
	text, err := r.ReadString()
	if err != nil || text != "" || r.Remaining() != 1 {
		t.Fatalf("null string framing: %q, error %v, remaining %d", text, err, r.Remaining())
	}
	unknownLength := append([]byte(nil), csharpFixture(t, "utf8")...)
	copy(unknownLength[4:8], []byte{255, 255, 255, 255})
	if err = memorypack.Deserialize(unknownLength, &text); err != nil || text != value {
		t.Fatalf("unknown UTF-16 length: %q, error %v", text, err)
	}
	invalid := [][]byte{
		{1, 0, 0, 0},
		{1, 0, 0, 0, 0, 0xd8},
		{1, 0, 0, 0, 0, 0xdc},
		{0xfe, 255, 255, 255, 1, 0, 0, 0, 255},
		{0xfe, 255, 255, 255, 2, 0, 0, 0, 'a'},
		{0xfe, 255, 255, 255, 0xfe, 255, 255, 255, 'a'},
	}
	for _, wire := range invalid {
		if err = memorypack.Deserialize(wire, &text); err == nil {
			t.Errorf("accepted invalid string %X", wire)
		}
	}
	if _, err = memorypack.Serialize(string([]byte{255})); err == nil {
		t.Fatal("accepted invalid Go UTF-8 string")
	}
}

func TestReaderRejectsMalformedContainers(t *testing.T) {
	for _, data := range [][]byte{{0xfe, 255, 255, 255}, {255, 255, 255, 127}, {1, 0, 16, 0}} {
		var slice []int64
		if err := memorypack.Deserialize(data, &slice); err == nil {
			t.Errorf("accepted invalid collection count %X", data)
		}
	}
	var array [1]int32
	for _, data := range [][]byte{{2, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0}, {0, 0, 0, 0}} {
		if err := memorypack.Deserialize(data, &array); err == nil {
			t.Errorf("accepted wrong array length %X", data)
		}
	}
	for _, header := range []byte{250, 251, 252, 253, 254} {
		var value struct{}
		if err := memorypack.Deserialize([]byte{header}, &value); err == nil {
			t.Errorf("accepted reserved object header %d", header)
		}
	}
	var mapping map[int32]int32
	duplicate := []byte{2, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0, 3, 0, 0, 0}
	if err := memorypack.Deserialize(duplicate, &mapping); err == nil {
		t.Fatal("accepted duplicate dictionary key")
	}
	r := memorypack.NewReader([]byte{1})
	if _, err := r.Peek(-1); err == nil || r.Remaining() != 1 {
		t.Fatalf("negative peek: error %v, remaining %d", err, r.Remaining())
	}
	if err := r.ReadValue((*int)(nil)); err == nil {
		t.Fatal("accepted nil destination pointer")
	}
	if err := r.ReadValue(nil); err == nil {
		t.Fatal("accepted nil destination interface")
	}
}

func TestReaderCollectionLimit(t *testing.T) {
	data := []byte{2, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0}
	var values []int32
	r := memorypack.NewReader(data)
	r.CollectionLimit = 1
	if err := r.ReadValue(&values); err == nil {
		t.Fatal("accepted a collection above the caller's limit")
	}
	r = memorypack.NewReader(data)
	r.CollectionLimit = 2
	if err := r.ReadValue(&values); err != nil || !reflect.DeepEqual(values, []int32{1, 2}) {
		t.Fatalf("exact-limit collection: %v, %v", values, err)
	}
}

func TestInvalidFieldOrders(t *testing.T) {
	values := []any{
		struct {
			Value int32 `memorypack:"bad"`
		}{},
		struct {
			Value int32 `memorypack:"-2"`
		}{},
		struct {
			First  int32 `memorypack:"0"`
			Second int32 `memorypack:"0"`
		}{},
	}
	for _, value := range values {
		if _, err := memorypack.Serialize(value); err == nil {
			t.Errorf("accepted invalid field orders on %T", value)
		}
	}
}

func FuzzDecode(f *testing.F) {
	if err := registerMessages(); err != nil {
		f.Fatal(err)
	}
	seeds := [][]byte{
		nil, {255}, {0}, {0, 0}, {1, 0, 0, 0}, {250, 250, 0, 1}, {1, 0x84, 0x80, 0}, {255, 255, 255, 127},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			return
		}
		var text string
		var slice []int64
		var array [2]uint16
		var mapping map[int32]string
		var union message
		var version newVersion
		var nullable memorypack.Nullable[int64]
		for _, target := range []any{&text, &slice, &array, &mapping, &union, &version, &nullable} {
			r := memorypack.NewReader(data)
			_ = r.ReadValue(target)
			if r.Remaining() < 0 || r.Remaining() > len(data) {
				t.Fatalf("decoder escaped input bounds for %T", target)
			}
		}
	})
}

// TestWriter tests the Writer class directly.
func TestWriter(t *testing.T) {
	t.Run("EnsureCapacity", func(t *testing.T) {
		// Start with a small buffer and write more than capacity
		writer := memorypack.NewWriter(2)

		// Write enough bytes to trigger capacity increase
		for i := range 100 {
			writer.WriteByte(byte(i))
		}

		// Verify the bytes were written correctly
		bytes := writer.GetBytes()
		if len(bytes) != 100 {
			t.Errorf("Expected 100 bytes, got %d", len(bytes))
		}

		for i := range 100 {
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
