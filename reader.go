package memorypack

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

// Deserialize deserializes a value from a byte slice.
//
// value must be a pointer to a value.
//
// If the value implements the Formatter interface, it will be used to deserialize.
//
// Otherwise, the value will be deserialized using reflection.
func Deserialize[T any](data []byte, value T) error {
	reader := NewReader(data)

	// Use reflection to check if value implements Formatter
	formatter, ok := any(value).(Formatter)
	if ok {
		if err := formatter.Deserialize(reader); err != nil {
			return fmt.Errorf("deserialize failed: %w", err)
		}
		return nil
	}

	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("deserialize requires a pointer to a value")
	}
	v = v.Elem()

	if v.Kind() == reflect.Struct {
		if err := deserializeStruct(reader, value); err != nil {
			return err
		}
	} else {
		if err := readValue(reader, v); err != nil {
			return err
		}
	}

	return nil
}

// Reader handles deserialization of data from a binary format.
type Reader struct {
	buffer []byte
	pos    int
}

// NewReader creates a new MemoryPack reader.
func NewReader(data []byte) *Reader {
	return &Reader{
		buffer: data,
		pos:    0,
	}
}

// ReadFormatVersion reads the MemoryPack format version.
func (r *Reader) ReadFormatVersion() (byte, error) {
	return r.ReadByte()
}

// ReadByte reads a byte from the buffer.
func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.buffer) {
		return 0, fmt.Errorf("cannot read byte: end of buffer")
	}

	v := r.buffer[r.pos]
	r.pos++
	return v, nil
}

// Peek reads the next n bytes without advancing the position.
func (r *Reader) Peek(n int) ([]byte, error) {
	if r.pos+n > len(r.buffer) {
		return nil, fmt.Errorf("cannot peek %d bytes: end of buffer", n)
	}

	return r.buffer[r.pos : r.pos+n], nil
}

// ReadBytes reads a byte slice from the buffer.
func (r *Reader) ReadBytes() ([]byte, error) {
	length, err := r.ReadInt32()
	if err != nil {
		return nil, err
	}

	if length == NullCollection {
		return nil, nil
	}

	if length < 0 {
		return nil, fmt.Errorf("invalid byte array length: %d", length)
	}

	// Bounds check
	if int(length) > len(r.buffer)-r.pos {
		return nil, fmt.Errorf("read error: requested %d bytes but only %d bytes available",
			length, len(r.buffer)-r.pos)
	}

	result := make([]byte, length)
	copy(result, r.buffer[r.pos:r.pos+int(length)])
	r.pos += int(length)
	return result, nil
}

// ReadInt16 reads an int16 from the buffer.
func (r *Reader) ReadInt16() (int16, error) {
	if r.pos+2 > len(r.buffer) {
		return 0, fmt.Errorf("cannot read int16: end of buffer")
	}
	v := binary.LittleEndian.Uint16(r.buffer[r.pos:])
	r.pos += 2
	return int16(v), nil
}

// ReadInt32 reads an int32 from the buffer.
func (r *Reader) ReadInt32() (int32, error) {
	if r.pos+4 > len(r.buffer) {
		return 0, fmt.Errorf("cannot read int32: end of buffer")
	}
	v := binary.LittleEndian.Uint32(r.buffer[r.pos:])
	r.pos += 4
	return int32(v), nil
}

// ReadInt64 reads an int64 from the buffer.
func (r *Reader) ReadInt64() (int64, error) {
	if r.pos+8 > len(r.buffer) {
		return 0, fmt.Errorf("cannot read int64: end of buffer")
	}
	v := binary.LittleEndian.Uint64(r.buffer[r.pos:])
	r.pos += 8
	return int64(v), nil
}

// ReadFloat32 reads a float32 from the buffer.
func (r *Reader) ReadFloat32() (float32, error) {
	if r.pos+4 > len(r.buffer) {
		return 0, fmt.Errorf("cannot read float32: end of buffer")
	}
	v := binary.LittleEndian.Uint32(r.buffer[r.pos:])
	r.pos += 4
	return math.Float32frombits(v), nil
}

// ReadFloat64 reads a float64 from the buffer.
func (r *Reader) ReadFloat64() (float64, error) {
	if r.pos+8 > len(r.buffer) {
		return 0, fmt.Errorf("cannot read float64: end of buffer")
	}
	v := binary.LittleEndian.Uint64(r.buffer[r.pos:])
	r.pos += 8
	return math.Float64frombits(v), nil
}

// ReadBool reads a boolean from the buffer.
func (r *Reader) ReadBool() (bool, error) {
	b, err := r.ReadByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// ReadString reads a string from the buffer using MemoryPack format.
func (r *Reader) ReadString() (string, error) {
	// Read the header
	byteCount, err := r.ReadInt32()
	if err != nil {
		return "", err
	}

	// Check if it's a collection header (non-negative)
	if byteCount >= 0 {
		// It's either a null or empty string
		if byteCount == NullCollection {
			return "", nil // null string
		}
		return "", nil // empty string (length 0)
	}

	// It's a normal string, the byteCount is negated (~)
	actualByteCount := ^byteCount

	// Read the string length (UTF-16 length in C#)
	_, err = r.ReadInt32() // Skip this in Go since we don't need it
	if err != nil {
		return "", err
	}

	// Read the UTF-8 bytes
	if r.pos+int(actualByteCount) > len(r.buffer) {
		return "", fmt.Errorf("read error: requested %d bytes for string but only %d bytes available",
			actualByteCount, len(r.buffer)-r.pos)
	}

	str := string(r.buffer[r.pos : r.pos+int(actualByteCount)])
	r.pos += int(actualByteCount)
	return str, nil
}

// ReadCollectionHeader reads a collection header and returns the length.
func (r *Reader) ReadCollectionHeader() (int, bool, error) {
	length, err := r.ReadInt32()
	if err != nil {
		return 0, false, err
	}
	if length == NullCollection {
		return 0, true, nil // null collection
	}
	return int(length), false, nil // non-null collection
}

// ReadObjectHeader reads an object header.
func (r *Reader) ReadObjectHeader() (int, bool, error) {
	header, err := r.ReadByte()
	if err != nil {
		return 0, false, err
	}
	if header == NullObject {
		return 0, true, nil // null object
	}
	return int(header), false, nil // member count
}
