package memorypack

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

// Serialize serializes any value into bytes.
func Serialize(value any) ([]byte, error) {
	writer := NewWriter(128)

	// Start with format version byte like C#
	if formatter, ok := value.(Formatter); ok {
		if err := formatter.Serialize(writer); err != nil {
			return nil, fmt.Errorf("failed to serialize value: %w", err)
		}
	}
	v := reflect.ValueOf(value)
	// Handle nil pointers explicitly
	if v.Kind() == reflect.Ptr && v.IsNil() {
		writer.WriteByte(NullObject)
	} else {
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			if err := SerializeStruct(writer, v.Interface()); err != nil {
				return nil, err
			}
		} else {
			if err := writeValue(writer, v); err != nil {
				return nil, err
			}
		}
	}

	return writer.GetBytes(), nil
}

// Writer handles serialization of data to a binary format.
type Writer struct {
	buffer []byte
	pos    int
	depth  int
}

// NewWriter creates a new MemoryPack writer with an optional initial capacity.
func NewWriter(initialCapacity int) *Writer {
	if initialCapacity <= 0 {
		initialCapacity = 64
	}
	return &Writer{
		buffer: make([]byte, initialCapacity),
		pos:    0,
	}
}

// CheckDepth increments the depth counter and checks for circular references.
func (w *Writer) CheckDepth() error {
	w.depth++
	if w.depth > MaxDepth {
		return fmt.Errorf("serialization depth exceeded %d, possible circular reference detected", MaxDepth)
	}
	return nil
}

// EndCheckDepth decrements the depth counter after serialization is complete.
func (w *Writer) EndCheckDepth() {
	w.depth--
}

// GetBytes returns the serialized bytes.
func (w *Writer) GetBytes() []byte {
	return w.buffer[:w.pos]
}

// ensureCapacity ensures the buffer has enough capacity.
func (w *Writer) ensureCapacity(additionalBytes int) {
	requiredCapacity := w.pos + additionalBytes
	if requiredCapacity > len(w.buffer) {
		newCapacity := len(w.buffer) * 2
		if newCapacity < requiredCapacity {
			newCapacity = requiredCapacity
		}
		newBuffer := make([]byte, newCapacity)
		copy(newBuffer, w.buffer)
		w.buffer = newBuffer
	}
}

// WriteFormatVersion writes the MemoryPack format version.
func (w *Writer) WriteFormatVersion() {
	w.WriteByte(MemoryPackFormatVersion)
}

// WriteByte writes a byte to the buffer.
func (w *Writer) WriteByte(v byte) {
	w.ensureCapacity(1)
	w.buffer[w.pos] = v
	w.pos++
}

// WriteBytes writes a byte slice to the buffer.
func (w *Writer) WriteBytes(v []byte) {
	if v == nil {
		// Null byte array
		w.WriteInt32(NullCollection)
		return
	}

	// Write the length
	w.WriteInt32(int32(len(v)))

	// Write the bytes
	if len(v) > 0 {
		w.ensureCapacity(len(v))
		copy(w.buffer[w.pos:], v)
		w.pos += len(v)
	}
}

// WriteInt16 writes an int16 to the buffer.
func (w *Writer) WriteInt16(v int16) {
	w.ensureCapacity(2)
	binary.LittleEndian.PutUint16(w.buffer[w.pos:], uint16(v))
	w.pos += 2
}

// WriteInt32 writes an int32 to the buffer.
func (w *Writer) WriteInt32(v int32) {
	w.ensureCapacity(4)
	binary.LittleEndian.PutUint32(w.buffer[w.pos:], uint32(v))
	w.pos += 4
}

// WriteInt64 writes an int64 to the buffer.
func (w *Writer) WriteInt64(v int64) {
	w.ensureCapacity(8)
	binary.LittleEndian.PutUint64(w.buffer[w.pos:], uint64(v))
	w.pos += 8
}

// WriteFloat32 writes a float32 to the buffer.
func (w *Writer) WriteFloat32(v float32) {
	w.ensureCapacity(4)
	binary.LittleEndian.PutUint32(w.buffer[w.pos:], math.Float32bits(v))
	w.pos += 4
}

// WriteFloat64 writes a float64 to the buffer.
func (w *Writer) WriteFloat64(v float64) {
	w.ensureCapacity(8)
	binary.LittleEndian.PutUint64(w.buffer[w.pos:], math.Float64bits(v))
	w.pos += 8
}

// WriteBool writes a boolean to the buffer.
func (w *Writer) WriteBool(v bool) {
	if v {
		w.WriteByte(1)
	} else {
		w.WriteByte(0)
	}
}

// WriteString writes a string to the buffer using MemoryPack format.
func (w *Writer) WriteString(v string) {
	if v == "" {
		// Empty string - write zero collection header
		w.WriteInt32(0)
		return
	}

	// Convert string to UTF-8 bytes
	utf8Bytes := []byte(v)
	utf8ByteCount := len(utf8Bytes)

	// Ensure we have enough capacity
	w.ensureCapacity(utf8ByteCount + 8) // data + 2 headers

	// Write negated UTF-8 byte count (~utf8-byte-count)
	w.WriteInt32(^int32(utf8ByteCount))

	// Write string length (UTF-16 code units in C#, chars in Go)
	w.WriteInt32(int32(len(v)))

	// Write the actual UTF-8 bytes
	copy(w.buffer[w.pos:], utf8Bytes)
	w.pos += utf8ByteCount
}

// WriteCollectionHeader writes a collection header (used for arrays, lists, etc).
func (w *Writer) WriteCollectionHeader(length int) {
	w.WriteInt32(int32(length))
}

// WriteNullCollectionHeader writes a null collection header.
func (w *Writer) WriteNullCollectionHeader() {
	w.WriteInt32(NullCollection)
}

// WriteObjectHeader writes an object header.
func (w *Writer) WriteObjectHeader(memberCount int) error {
	switch {
	case memberCount < 0:
		w.WriteByte(NullObject)
	case memberCount <= 249:
		w.WriteByte(byte(memberCount))
	default:
		return fmt.Errorf("member count too large: %d (max 249)", memberCount)
	}
	return nil
}
