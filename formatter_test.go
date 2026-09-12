package memorypack_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

func TestCustomFormatterExactBytes(t *testing.T) {
	value := &CustomFormat{IntValue: 42, StrValue: "x"}
	want := []byte{42, 0, 0, 0, 0xfe, 0xff, 0xff, 0xff, 1, 0, 0, 0, 'x'}
	got, err := memorypack.Serialize(value)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("custom format: got %X, want %X, error %v", got, want, err)
	}
}

func TestCustomFormatterCollections(t *testing.T) {
	value := []CustomFormat{{IntValue: 255, StrValue: "x"}}
	want := []byte{1, 0, 0, 0, 255, 0, 0, 0, 0xfe, 0xff, 0xff, 0xff, 1, 0, 0, 0, 'x'}
	got, err := memorypack.Serialize(value)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("slice custom bytes: got %X, want %X, error %v", got, want, err)
	}
	var decoded []CustomFormat
	if err = memorypack.Deserialize(want, &decoded); err != nil || !reflect.DeepEqual(decoded, value) {
		t.Fatalf("slice custom decode: got %+v, error %v", decoded, err)
	}
	mapValue := map[int32]CustomFormat{1: value[0]}
	// A one-element dictionary starts with its count and then its int32 key.
	want = append([]byte{1, 0, 0, 0}, want...)
	got, err = memorypack.Serialize(mapValue)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("map custom bytes: got %X, want %X, error %v", got, want, err)
	}
	var decodedMap map[int32]CustomFormat
	if err = memorypack.Deserialize(want, &decodedMap); err != nil || !reflect.DeepEqual(decodedMap, mapValue) {
		t.Fatalf("map custom decode: got %+v, error %v", decodedMap, err)
	}
}

type failingFormat struct{ failure error }

func (f failingFormat) MarshalMemoryPack(*memorypack.Writer) error    { return f.failure }
func (f *failingFormat) UnmarshalMemoryPack(*memorypack.Reader) error { return f.failure }

type (
	encodeOnly int32
	decodeOnly int32
)

func (v encodeOnly) MarshalMemoryPack(w *memorypack.Writer) error {
	w.WriteByte(byte(v))
	return nil
}

func (v *decodeOnly) UnmarshalMemoryPack(r *memorypack.Reader) error {
	b, err := r.ReadByte()
	*v = decodeOnly(b)
	return err
}

func TestFormatterCapabilitiesAndErrors(t *testing.T) {
	data, err := memorypack.Serialize(encodeOnly(42))
	if err != nil || !bytes.Equal(data, []byte{42}) {
		t.Fatalf("one-way marshal: %X, %v", data, err)
	}
	var result decodeOnly
	if err = memorypack.Deserialize([]byte{42}, &result); err != nil || result != 42 {
		t.Fatalf("one-way unmarshal: %d, %v", result, err)
	}
	failure := errors.New("formatter failed")
	value := struct{ Custom failingFormat }{Custom: failingFormat{failure: failure}}
	if _, err = memorypack.Serialize(&value); !errors.Is(err, failure) {
		t.Fatalf("lost custom serialize error: %v", err)
	}
	if err = memorypack.Deserialize([]byte{1}, &value); !errors.Is(err, failure) {
		t.Fatalf("lost custom deserialize error: %v", err)
	}
}

type recursiveFormat struct{}

func (v *recursiveFormat) MarshalMemoryPack(w *memorypack.Writer) error   { return w.WriteValue(v) }
func (v *recursiveFormat) UnmarshalMemoryPack(r *memorypack.Reader) error { return r.ReadValue(v) }

func TestFormatterDepthAndReuse(t *testing.T) {
	value := &recursiveFormat{}
	w := memorypack.NewWriter(1)
	if err := w.WriteValue(value); err == nil {
		t.Fatal("custom writer bypassed depth limit")
	}
	if err := w.WriteValue(int32(42)); err != nil {
		t.Fatalf("writer depth did not recover after error: %v", err)
	}
	r := memorypack.NewReader([]byte{42, 0, 0, 0})
	if err := r.ReadValue(value); err == nil {
		t.Fatal("custom reader bypassed depth limit")
	}
	var number int32
	if err := r.ReadValue(&number); err != nil || number != 42 {
		t.Fatalf("reader did not recover after error: %d, %v", number, err)
	}
}

func TestNestedCustomFormatter(t *testing.T) {
	value := struct{ Custom CustomFormat }{Custom: CustomFormat{IntValue: 255, StrValue: "x"}}
	want := []byte{1, 255, 0, 0, 0, 0xfe, 0xff, 0xff, 0xff, 1, 0, 0, 0, 'x'}
	got, err := memorypack.Serialize(value)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("nested custom format: got %X, want %X, error %v", got, want, err)
	}
	var decoded struct{ Custom CustomFormat }
	if err = memorypack.Deserialize(want, &decoded); err != nil || decoded != value {
		t.Fatalf("nested custom decode: got %+v, error %v", decoded, err)
	}
}

type taggedByte byte

func (v taggedByte) MarshalMemoryPack(w *memorypack.Writer) error {
	w.WriteByte(0xaa)
	w.WriteByte(byte(v))
	return nil
}

func (v *taggedByte) UnmarshalMemoryPack(r *memorypack.Reader) error {
	marker, err := r.ReadByte()
	if err != nil {
		return err
	}
	if marker != 0xaa {
		return fmt.Errorf("invalid tagged byte marker: %d", marker)
	}
	value, err := r.ReadByte()
	*v = taggedByte(value)
	return err
}

func TestByteElementsRespectCustomFormatters(t *testing.T) {
	value := []taggedByte{7}
	want := []byte{1, 0, 0, 0, 0xaa, 7}
	data, err := memorypack.Serialize(value)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("byte slice bypassed formatter: %X, error %v", data, err)
	}
	var decoded []taggedByte
	if err = memorypack.Deserialize(want, &decoded); err != nil || !reflect.DeepEqual(decoded, value) {
		t.Fatalf("byte slice decode: %v, error %v", decoded, err)
	}
}

var _ memorypack.Formatter = (*CustomFormat)(nil)
