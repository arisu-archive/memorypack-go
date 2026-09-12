package memorypack_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

type oldVersion struct {
	Value   int32  `memorypack:"0"`
	Removed int64  `memorypack:"1"`
	Text    string `memorypack:"2"`
}

type newVersion struct {
	Value int32  `memorypack:"0"`
	Text  string `memorypack:"2"`
	Added int16  `memorypack:"3"`
}

type longVersion struct {
	Text string `memorypack:"0"`
}

type missingOrder struct{ Value int32 }

func (oldVersion) MemoryPackVersionTolerant()   {}
func (*newVersion) MemoryPackVersionTolerant()  {}
func (longVersion) MemoryPackVersionTolerant()  {}
func (missingOrder) MemoryPackVersionTolerant() {}

func TestVersionTolerantCSharpFixtures(t *testing.T) {
	checkCSharpFixture(t, "version-old", oldVersion{Value: 42, Removed: 999, Text: "hello"})
	checkCSharpFixture(t, "version-new", newVersion{Value: 42, Text: "hello", Added: 7})
	checkCSharpFixture(t, "version-long", longVersion{Text: strings.Repeat("a", 120)})

	newValue := newVersion{Added: 100}
	if err := memorypack.Deserialize(csharpFixture(t, "version-old"), &newValue); err != nil {
		t.Fatal(err)
	}
	if newValue != (newVersion{Value: 42, Text: "hello", Added: 100}) {
		t.Fatalf("old -> new schema: got %+v", newValue)
	}
	oldValue := oldVersion{Removed: 100}
	if err := memorypack.Deserialize(csharpFixture(t, "version-new"), &oldValue); err != nil {
		t.Fatal(err)
	}
	if oldValue != (oldVersion{Value: 42, Removed: 100, Text: "hello"}) {
		t.Fatalf("new -> old schema: got %+v", oldValue)
	}
	// The removed member contains an object, not a guessed primitive shape.
	wire := []byte{4, 4, 3, 0, 2, 42, 0, 0, 0, 2, 1, 0, 7, 0}
	if err := memorypack.Deserialize(wire, &newValue); err != nil {
		t.Fatal(err)
	}
	if newValue != (newVersion{Value: 42, Text: "hello", Added: 7}) {
		t.Fatalf("skipping an unknown member lost alignment: %+v", newValue)
	}
}

func TestObjectSchemaEvolution(t *testing.T) {
	value := struct {
		Value int32
		Text  string
	}{Text: "old state"}
	if err := memorypack.Deserialize(csharpFixture(t, "object-old"), &value); err != nil {
		t.Fatal(err)
	}
	if value.Value != 42 || value.Text != "old state" {
		t.Fatalf("missing fields must retain their existing value: %+v", value)
	}
	old := struct{ Value int32 }{}
	if err := memorypack.Deserialize(csharpFixture(t, "object-new"), &old); err == nil {
		t.Fatal("ordinary objects cannot skip unknown fields without lengths")
	}
}

func TestVersionTolerantRejectsInvalidLengths(t *testing.T) {
	cases := [][]byte{
		{1},
		{1, 0xff},
		{1, 0x84},
		{1, 0x82, 0xff, 0xff, 0xff, 0x7f},
		{1, 3, 42, 0, 0},
		{1, 5, 42, 0, 0, 0, 99},
		{2, 4, 4, 42, 0, 0, 0},
	}
	for _, data := range cases {
		var value newVersion
		if err := memorypack.Deserialize(data, &value); err == nil {
			t.Errorf("accepted invalid field framing: %X", data)
		}
	}
	valid := csharpFixture(t, "version-old")
	for length := range valid {
		var value oldVersion
		if err := memorypack.Deserialize(valid[:length], &value); err == nil {
			t.Errorf("accepted version-tolerant object truncated at %d", length)
		}
	}
	if _, err := memorypack.Serialize(missingOrder{}); err == nil {
		t.Fatal("accepted version-tolerant field without explicit order")
	}
}

func TestMemoryPackVarInts(t *testing.T) {
	cases := []struct {
		value int32
		wire  []byte
	}{
		{0, []byte{0}},
		{127, []byte{127}},
		{-120, []byte{0x88}},
		{-121, []byte{0x86, 0x87}},
		{128, []byte{0x84, 0x80, 0}},
		{32767, []byte{0x84, 0xff, 0x7f}},
		{32768, []byte{0x82, 0, 0x80, 0, 0}},
		{-32768, []byte{0x84, 0, 0x80}},
		{-2147483648, []byte{0x82, 0, 0, 0, 0x80}},
		{2147483647, []byte{0x82, 0xff, 0xff, 0xff, 0x7f}},
	}
	for _, tc := range cases {
		writer := memorypack.NewWriter(1)
		writer.WriteVarInt(tc.value)
		if !bytes.Equal(writer.GetBytes(), tc.wire) {
			t.Errorf("varint %d: got %X, want %X", tc.value, writer.GetBytes(), tc.wire)
		}
		value, err := memorypack.NewReader(tc.wire).ReadVarInt()
		if err != nil || value != tc.value {
			t.Errorf("varint %X: got %d, error %v", tc.wire, value, err)
		}
	}
	overflows := [][]byte{
		{0x83, 0xff, 0xff, 0xff, 0xff},
		{0x81, 0, 0, 0, 0x80, 0, 0, 0, 0},
		{0x80, 0, 0, 0, 0, 1, 0, 0, 0},
	}
	for _, data := range overflows {
		if _, err := memorypack.NewReader(data).ReadVarInt(); err == nil {
			t.Errorf("accepted varint overflow %X", data)
		}
	}
}
