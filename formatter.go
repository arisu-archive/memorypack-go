package memorypack

import "reflect"

// Marshaler supplies the complete wire representation of a value. It is used
// at every nesting level, including map values. Implementations must propagate
// errors from nested writes and must not retain the writer.
type Marshaler interface {
	MarshalMemoryPack(writer *Writer) error
}

// Unmarshaler reads a value from the current reader position. Implementations
// must consume exactly their value's bytes and must not retain the reader.
type Unmarshaler interface {
	UnmarshalMemoryPack(reader *Reader) error
}

// Formatter combines custom serialization and deserialization. Implementations
// may also be used through either one-way capability.
type Formatter interface {
	Marshaler
	Unmarshaler
}

func marshalerFor(v reflect.Value) (Marshaler, bool) {
	if v.CanInterface() {
		if m, ok := v.Interface().(Marshaler); ok {
			return m, true
		}
	}
	if v.Kind() == reflect.Ptr || !reflect.PointerTo(v.Type()).Implements(reflect.TypeFor[Marshaler]()) {
		return nil, false
	}
	if !v.CanAddr() {
		copyValue := reflect.New(v.Type()).Elem()
		copyValue.Set(v)
		v = copyValue
	}
	m, ok := v.Addr().Interface().(Marshaler)
	return m, ok
}

func unmarshalerFor(v reflect.Value) (Unmarshaler, bool) {
	if v.CanAddr() && v.Addr().CanInterface() {
		if m, ok := v.Addr().Interface().(Unmarshaler); ok {
			return m, true
		}
	}
	return nil, false
}
