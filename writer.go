package memorypack

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"unicode/utf16"
)

// StringEncoding selects the representation of strings on the wire.
type StringEncoding uint8

const (
	// UTF8 is the default C# MemoryPack string encoding.
	UTF8 StringEncoding = iota
	// UTF16 writes little-endian UTF-16 code units.
	UTF16
)

// Options configures serialization. Its zero value uses UTF-8.
type Options struct {
	StringEncoding StringEncoding
}

// Serialize encodes a value. Pass &value to preserve an interface's union schema
// or a nullable pointer's type. The outer pointer is an address, not a wire value.
func Serialize(value any) ([]byte, error) {
	return SerializeWithOptions(value, Options{})
}

// SerializeWithOptions encodes a value using the selected string encoding.
func SerializeWithOptions(value any, options Options) ([]byte, error) {
	if options.StringEncoding != UTF8 && options.StringEncoding != UTF16 {
		return nil, fmt.Errorf("invalid string encoding: %d", options.StringEncoding)
	}
	writer := NewWriter(initialWriterCapacity)
	writer.stringEncoding = options.StringEncoding
	if err := writer.WriteValue(value); err != nil {
		return nil, err
	}
	return writer.GetBytes(), nil
}

// Writer handles serialization of data to a binary format.
type Writer struct {
	buffer         []byte
	pos            int
	depth          int
	stringEncoding StringEncoding
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
	if w.depth >= MaxDepth {
		return fmt.Errorf("serialization depth exceeded %d, possible circular reference detected", MaxDepth)
	}
	w.depth++
	return nil
}

// EndCheckDepth decrements the depth counter after serialization is complete.
func (w *Writer) EndCheckDepth() {
	w.depth--
}

// GetBytes returns bytes that alias the writer's buffer. Copy them before reusing
// the writer if independent ownership is required. Writer is not concurrency safe.
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
// Direct callers must supply valid UTF-8 with a byte length no greater than
// math.MaxInt32. WriteValue validates these preconditions and returns an error.
func (w *Writer) WriteString(v string) {
	if w.stringEncoding == UTF16 {
		w.WriteStringUTF16(v)
		return
	}
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

	// UTF-16 code units cannot outnumber UTF-8 bytes under WriteString's preconditions.
	w.WriteInt32(int32(utf16Length(v))) //nolint:gosec // WriteValue bounds the byte length to int32.

	// Write the actual UTF-8 bytes
	copy(w.buffer[w.pos:], utf8Bytes)
	w.pos += utf8ByteCount
}

// WriteStringUTF16 writes a string using C# MemoryPack's UTF-16 representation.
// Direct callers have the same input preconditions as WriteString.
func (w *Writer) WriteStringUTF16(v string) {
	units := utf16.Encode([]rune(v))
	w.WriteCollectionHeader(len(units))
	for _, unit := range units {
		w.WriteUint16(unit)
	}
}

func utf16Length(value string) int {
	length := 0
	for _, r := range value {
		length++
		if r > math.MaxUint16 {
			length++
		}
	}
	return length
}

// WriteValue appends a value using the same address and schema rules as Serialize.
// Custom formatters can use it to encode nested values on the current writer.
func (w *Writer) WriteValue(value any) error {
	v := reflect.ValueOf(value)
	if v.IsValid() && v.Kind() == reflect.Ptr && !v.IsNil() {
		if _, ok := value.(Marshaler); !ok {
			v = v.Elem()
		}
	}
	return writeValue(w, v)
}

// WriteUint16 writes an unsigned 16-bit integer in little-endian order.
func (w *Writer) WriteUint16(value uint16) {
	w.ensureCapacity(int16Size)
	binary.LittleEndian.PutUint16(w.buffer[w.pos:], value)
	w.pos += int16Size
}

// WriteUint32 writes an unsigned 32-bit integer in little-endian order.
func (w *Writer) WriteUint32(value uint32) {
	w.ensureCapacity(int32Size)
	binary.LittleEndian.PutUint32(w.buffer[w.pos:], value)
	w.pos += int32Size
}

// WriteUint64 writes an unsigned 64-bit integer in little-endian order.
func (w *Writer) WriteUint64(value uint64) {
	w.ensureCapacity(int64Size)
	binary.LittleEndian.PutUint64(w.buffer[w.pos:], value)
	w.pos += int64Size
}

// WriteRaw appends bytes without a length prefix and copies the input.
func (w *Writer) WriteRaw(data []byte) {
	w.ensureCapacity(len(data))
	w.pos += copy(w.buffer[w.pos:], data)
}

// WriteUnionHeader writes a tag, using the wide representation for tags >= 250.
func (w *Writer) WriteUnionHeader(tag uint16) {
	if tag < uint16(WideTag) {
		w.WriteByte(byte(tag))
		return
	}
	w.WriteByte(WideTag)
	w.WriteUint16(tag)
}

// WriteNullUnionHeader writes a null union value without a payload.
func (w *Writer) WriteNullUnionHeader() { w.WriteByte(NullObject) }

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
