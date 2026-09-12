package memorypack

import (
	"errors"
	"fmt"
	"math"
	"reflect"
)

const (
	varIntUInt8  byte = 0x87
	varIntInt8   byte = 0x86
	varIntUInt16 byte = 0x85
	varIntInt16  byte = 0x84
	varIntUInt32 byte = 0x83
	varIntInt32  byte = 0x82
	varIntUInt64 byte = 0x81
	varIntInt64  byte = 0x80
)

// VersionTolerant opts a struct into C# GenerateType.VersionTolerant encoding.
// Implement the marker on the struct or its pointer. Every serialized field
// needs an explicit memorypack order from 0 to 248. Orders may have gaps but
// must never be reused for a different field or wire type across schema versions.
type VersionTolerant interface {
	MemoryPackVersionTolerant()
}

func writeVersionTolerant(w *Writer, value reflect.Value, fd formatterData) error {
	payload := NewWriter(initialWriterCapacity)
	payload.depth = w.depth
	payload.stringEncoding = w.stringEncoding
	lengths := make([]int32, fd.memberCount)
	for _, field := range fd.fields {
		start := payload.pos
		if err := writeValue(payload, value.Field(field.index)); err != nil {
			return fmt.Errorf("%s.%s: %w", value.Type(), field.name, err)
		}
		length := payload.pos - start
		if length < 0 || length > math.MaxInt32 {
			return fmt.Errorf("version-tolerant field %s exceeds int32 length", field.name)
		}
		lengths[field.order] = int32(length)
	}
	if err := w.WriteObjectHeader(fd.memberCount); err != nil {
		return err
	}
	for _, length := range lengths {
		w.WriteVarInt(length)
	}
	w.WriteRaw(payload.GetBytes())
	return nil
}

func readVersionTolerant(r *Reader, value reflect.Value, fd formatterData, count int) error {
	lengths := make([]int, count)
	total := 0
	for i := range lengths {
		length, err := r.ReadVarInt()
		if err != nil {
			return err
		}
		if length < 0 || int(length) > r.Remaining()-total {
			return fmt.Errorf("invalid version-tolerant field length: %d", length)
		}
		lengths[i] = int(length)
		total += int(length)
	}
	if total > r.Remaining() {
		return errors.New("truncated version-tolerant payload")
	}
	fieldIndex := 0
	for order, length := range lengths {
		data, err := r.ReadRaw(length)
		if err != nil {
			return err
		}
		if fieldIndex >= len(fd.fields) || fd.fields[fieldIndex].order != order {
			continue
		}
		field := fd.fields[fieldIndex]
		fieldIndex++
		if length == 0 {
			continue
		}
		fieldReader := NewReader(data)
		fieldReader.depth = r.depth
		fieldReader.CollectionLimit = r.CollectionLimit
		if err = readValue(fieldReader, value.Field(field.index)); err != nil {
			return fmt.Errorf("%s.%s: %w", value.Type(), field.name, err)
		}
		if fieldReader.Remaining() != 0 {
			return fmt.Errorf("%s.%s did not consume its declared field length", value.Type(), field.name)
		}
	}
	return nil
}

// WriteVarInt writes the C# MemoryPack signed int32 variable-length format.
// This is a type-code format, not LEB128 or the encoding/binary varint format.
//
//nolint:gosec // Narrowing preserves signed wire bits within the branch's checked range.
func (w *Writer) WriteVarInt(value int32) {
	switch {
	case value >= -120 && value <= 127:
		w.WriteByte(byte(value))
	case value >= math.MinInt8 && value <= math.MaxInt8:
		w.WriteByte(varIntInt8)
		w.WriteByte(byte(value))
	case value >= math.MinInt16 && value <= math.MaxInt16:
		w.WriteByte(varIntInt16)
		w.WriteInt16(int16(value))
	default:
		w.WriteByte(varIntInt32)
		w.WriteInt32(value)
	}
}

// ReadVarInt reads a MemoryPack variable-length integer and rejects values
// outside the int32 range, including wide unsigned representations.
//
//nolint:gosec // Reinterpret signed payload bits, then reject results outside the int32 range.
func (r *Reader) ReadVarInt() (int32, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if prefix <= 0x7f || prefix >= 0x88 {
		return int32(int8(prefix)), nil
	}
	var value int64
	switch prefix {
	case varIntUInt8, varIntInt8:
		var x byte
		x, err = r.ReadByte()
		value = int64(x)
		if prefix == varIntInt8 {
			value = int64(int8(x))
		}
	case varIntUInt16, varIntInt16:
		var x uint16
		x, err = r.ReadUint16()
		value = int64(x)
		if prefix == varIntInt16 {
			value = int64(int16(x))
		}
	case varIntUInt32, varIntInt32:
		var x uint32
		x, err = r.ReadUint32()
		value = int64(x)
		if prefix == varIntInt32 {
			value = int64(int32(x))
		}
	case varIntUInt64, varIntInt64:
		var x uint64
		x, err = r.ReadUint64()
		if err == nil && prefix == varIntUInt64 && x > math.MaxInt32 {
			return 0, errors.New("varint overflows int32")
		}
		value = int64(x)
	}
	if err != nil {
		return 0, err
	}
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, errors.New("varint overflows int32")
	}
	return int32(value), nil
}
