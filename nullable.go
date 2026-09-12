package memorypack

import (
	"errors"
	"fmt"
	"reflect"
)

// Scalar is a fixed-width numeric or boolean MemoryPack value. Go int and uint
// use 64 bits on the wire, as they do in the ordinary reflection codec.
type Scalar interface {
	~bool | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// Nullable represents C# Nullable<T> for a Scalar, including its alignment
// padding. A zero value is null; Value is ignored when HasValue is false.
// Unlike a root pointer, Nullable is always a wire value rather than an address.
type Nullable[T Scalar] struct {
	Value    T
	HasValue bool
}

// MarshalMemoryPack writes the nullable flag, padding and scalar payload.
func (n Nullable[T]) MarshalMemoryPack(w *Writer) error {
	writeNullable(w, reflect.ValueOf(n.Value), n.HasValue)
	return nil
}

// UnmarshalMemoryPack replaces the value, clearing Value when the wire is null.
func (n *Nullable[T]) UnmarshalMemoryPack(r *Reader) error {
	if n == nil {
		return errors.New("nil Nullable destination")
	}
	var value T
	hasValue, err := readNullable(r, reflect.ValueOf(&value).Elem())
	if err != nil {
		return err
	}
	n.Value, n.HasValue = value, hasValue
	return nil
}

func scalarSize(kind reflect.Kind) int {
	switch kind {
	case reflect.Bool, reflect.Int8, reflect.Uint8:
		return 1
	case reflect.Int16, reflect.Uint16:
		return int16Size
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		return int32Size
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Float64:
		return int64Size
	default:
		return 0
	}
}

func writeNullable(w *Writer, value reflect.Value, present bool) {
	w.WriteBool(present)
	for i := 1; i < scalarSize(value.Kind()); i++ {
		w.WriteByte(0)
	}
	if !present {
		value = reflect.Zero(value.Type())
	}
	writeScalar(w, value)
}

func readNullable(r *Reader, value reflect.Value) (bool, error) {
	header, err := r.ReadRaw(scalarSize(value.Kind()))
	if err != nil {
		return false, err
	}
	if header[0] > 1 {
		return false, fmt.Errorf("invalid nullable flag: %d", header[0])
	}
	if err = readScalar(r, value); err != nil {
		return false, err
	}
	if header[0] == 0 {
		value.SetZero()
		return false, nil
	}
	return true, nil
}

//nolint:gosec // Each reflect.Kind fixes the range of its corresponding narrowing conversion.
func writeScalar(w *Writer, v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		w.WriteBool(v.Bool())
	case reflect.Int8:
		w.WriteByte(byte(v.Int()))
	case reflect.Int16:
		w.WriteInt16(int16(v.Int()))
	case reflect.Int32:
		w.WriteInt32(int32(v.Int()))
	case reflect.Int, reflect.Int64:
		w.WriteInt64(v.Int())
	case reflect.Uint8:
		w.WriteByte(byte(v.Uint()))
	case reflect.Uint16:
		w.WriteUint16(uint16(v.Uint()))
	case reflect.Uint32:
		w.WriteUint32(uint32(v.Uint()))
	case reflect.Uint, reflect.Uint64:
		w.WriteUint64(v.Uint())
	case reflect.Float32:
		w.WriteFloat32(float32(v.Float()))
	case reflect.Float64:
		w.WriteFloat64(v.Float())
	default:
		panic("writeScalar called with a non-scalar value")
	}
}

func readScalar(r *Reader, v reflect.Value) error {
	if v.Kind() == reflect.Bool {
		value, err := r.ReadBool()
		if err == nil {
			v.SetBool(value)
		}
		return err
	}
	if v.Kind() == reflect.Float32 || v.Kind() == reflect.Float64 {
		var value float64
		var err error
		if v.Kind() == reflect.Float32 {
			var f float32
			f, err = r.ReadFloat32()
			value = float64(f)
		} else {
			value, err = r.ReadFloat64()
		}
		if err == nil {
			v.SetFloat(value)
		}
		return err
	}
	return readInteger(r, v)
}

//nolint:gosec // Signed reinterpretation and sign extension preserve wire bits; destination overflow is checked.
func readInteger(r *Reader, v reflect.Value) error {
	size := scalarSize(v.Kind())
	data, err := r.ReadRaw(size)
	if err != nil {
		return err
	}
	var bits uint64
	for i, b := range data {
		bits |= uint64(b) << (bitsPerByte * i)
	}
	if v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64 {
		shift := uint((int64Size - size) * bitsPerByte)
		value := int64(bits<<shift) >> shift
		if v.OverflowInt(value) {
			return fmt.Errorf("integer overflows %s", v.Type())
		}
		v.SetInt(value)
	} else {
		if v.OverflowUint(bits) {
			return fmt.Errorf("integer overflows %s", v.Type())
		}
		v.SetUint(bits)
	}
	return nil
}
