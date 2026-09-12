package memorypack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"unicode/utf16"
	"unicode/utf8"
)

// Deserialize deserializes a value from a byte slice.
//
// value must be a pointer to a value.
//
// If the value implements Unmarshaler, its UnmarshalMemoryPack method is used.
//
// Otherwise, the value is decoded using reflection. Missing object fields retain
// their existing values; start with a zero value to default missing fields.
// Trailing bytes are allowed. Use Reader to consume consecutive values.
func Deserialize[T any](data []byte, value T) error {
	reader := NewReader(data)
	return reader.ReadValue(value)
}

// Reader handles deserialization of data from a binary format.
type Reader struct {
	// CollectionLimit is the largest accepted collection element count. Set it
	// before decoding; a negative limit is invalid. It does not limit byte slices.
	CollectionLimit int
	buffer          []byte
	pos             int
	depth           int
}

// NewReader creates a new MemoryPack reader.
func NewReader(data []byte) *Reader {
	return &Reader{
		CollectionLimit: DefaultCollectionLimit,
		buffer:          data,
		pos:             0,
	}
}

// ReadFormatVersion reads the MemoryPack format version.
func (r *Reader) ReadFormatVersion() (byte, error) {
	return r.ReadByte()
}

// ReadByte reads a byte from the buffer.
func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.buffer) {
		return 0, errors.New("cannot read byte: end of buffer")
	}

	v := r.buffer[r.pos]
	r.pos++
	return v, nil
}

// Peek reads the next n bytes without advancing the position.
func (r *Reader) Peek(n int) ([]byte, error) {
	if n < 0 || n > r.Remaining() {
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
		return 0, errors.New("cannot read int16: end of buffer")
	}
	v := binary.LittleEndian.Uint16(r.buffer[r.pos:])
	r.pos += 2
	return int16(v), nil //nolint:gosec // Reinterpret the signed wire bits without changing their representation.
}

// ReadInt32 reads an int32 from the buffer.
func (r *Reader) ReadInt32() (int32, error) {
	if r.pos+4 > len(r.buffer) {
		return 0, errors.New("cannot read int32: end of buffer")
	}
	v := binary.LittleEndian.Uint32(r.buffer[r.pos:])
	r.pos += 4
	return int32(v), nil //nolint:gosec // Reinterpret the signed wire bits without changing their representation.
}

// ReadInt64 reads an int64 from the buffer.
func (r *Reader) ReadInt64() (int64, error) {
	if r.pos+8 > len(r.buffer) {
		return 0, errors.New("cannot read int64: end of buffer")
	}
	v := binary.LittleEndian.Uint64(r.buffer[r.pos:])
	r.pos += 8
	return int64(v), nil //nolint:gosec // Reinterpret the signed wire bits without changing their representation.
}

// ReadFloat32 reads a float32 from the buffer.
func (r *Reader) ReadFloat32() (float32, error) {
	if r.pos+4 > len(r.buffer) {
		return 0, errors.New("cannot read float32: end of buffer")
	}
	v := binary.LittleEndian.Uint32(r.buffer[r.pos:])
	r.pos += 4
	return math.Float32frombits(v), nil
}

// ReadFloat64 reads a float64 from the buffer.
func (r *Reader) ReadFloat64() (float64, error) {
	if r.pos+8 > len(r.buffer) {
		return 0, errors.New("cannot read float64: end of buffer")
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
	length, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	if length == NullCollection || length == 0 {
		return "", nil
	}
	if length > 0 {
		return r.readUTF16(int(length))
	}
	units, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	data, err := r.ReadRaw(int(^length))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("invalid UTF-8 string")
	}
	value := string(data)
	if units < -1 || (units >= 0 && int(units) != utf16Length(value)) {
		return "", fmt.Errorf("invalid UTF-16 length in UTF-8 string: %d", units)
	}
	return value, nil
}

func (r *Reader) readUTF16(length int) (string, error) {
	if length > r.Remaining()/2 {
		return "", fmt.Errorf("truncated UTF-16 string: %d code units", length)
	}
	data, err := r.ReadRaw(length * int16Size)
	if err != nil {
		return "", err
	}
	units := make([]uint16, length)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	for i := 0; i < len(units); i++ {
		if units[i] >= 0xd800 && units[i] <= 0xdbff {
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", errors.New("unpaired UTF-16 high surrogate")
			}
			i++
		} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
			return "", errors.New("unpaired UTF-16 low surrogate")
		}
	}
	return string(utf16.Decode(units)), nil
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
	if length < 0 || r.CollectionLimit < 0 || int(length) > r.CollectionLimit {
		return 0, false, fmt.Errorf("invalid collection length: %d (limit %d)", length, r.CollectionLimit)
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
	if header >= WideTag {
		return 0, false, fmt.Errorf("reserved object header: %d", header)
	}
	return int(header), false, nil // member count
}

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int { return len(r.buffer) - r.pos }

// ReadRaw consumes length bytes without a header. The result borrows the input
// buffer; copy it before retaining or modifying it.
func (r *Reader) ReadRaw(length int) ([]byte, error) {
	data, err := r.Peek(length)
	if err != nil {
		return nil, err
	}
	r.pos += length
	return data, nil
}

// ReadValue decodes one value into a non-nil pointer, leaving subsequent bytes
// available. On error the destination may be partly updated. Reader is not safe
// for concurrent use, and its input must not be mutated during decoding.
func (r *Reader) ReadValue(value any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.New("deserialize requires a non-nil pointer to a value")
	}
	return readValue(r, v.Elem())
}

// ReadUint16 reads an unsigned 16-bit little-endian integer.
func (r *Reader) ReadUint16() (uint16, error) {
	data, err := r.ReadRaw(int16Size)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(data), nil
}

// ReadUint32 reads an unsigned 32-bit little-endian integer.
func (r *Reader) ReadUint32() (uint32, error) {
	data, err := r.ReadRaw(int32Size)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(data), nil
}

// ReadUint64 reads an unsigned 64-bit little-endian integer.
func (r *Reader) ReadUint64() (uint64, error) {
	data, err := r.ReadRaw(int64Size)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}

// ReadUnionHeader returns a tag or reports a null union. Reserved markers and
// truncated wide tags return errors.
func (r *Reader) ReadUnionHeader() (uint16, bool, error) {
	header, err := r.ReadByte()
	if err != nil {
		return 0, false, err
	}
	switch {
	case header < WideTag:
		return uint16(header), false, nil
	case header == WideTag:
		tag, readErr := r.ReadUint16()
		return tag, false, readErr
	case header == NullObject:
		return 0, true, nil
	default:
		return 0, false, fmt.Errorf("reserved union header: %d", header)
	}
}
