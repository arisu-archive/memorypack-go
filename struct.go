package memorypack

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"unicode/utf8"
)

//nolint:gochecknoglobals // Type metadata is shared process-wide and immutable once published under the mutex.
var formatterCache = struct {
	mu   sync.RWMutex
	data map[reflect.Type]formatterData
}{data: make(map[reflect.Type]formatterData)}

type formatterData struct {
	fields          []fieldInfo
	versionTolerant bool
	memberCount     int
	err             error
}

type fieldInfo struct {
	index int
	name  string
	order int
}

func serializeStruct(w *Writer, v reflect.Value) error {
	fd := getFormatterData(v.Type())
	if fd.err != nil {
		return fd.err
	}
	if fd.versionTolerant {
		return writeVersionTolerant(w, v, fd)
	}
	if err := w.WriteObjectHeader(len(fd.fields)); err != nil {
		return err
	}
	for _, field := range fd.fields {
		if err := writeValue(w, v.Field(field.index)); err != nil {
			return fmt.Errorf("%s.%s: %w", v.Type(), field.name, err)
		}
	}
	return nil
}

func deserializeStruct(r *Reader, v reflect.Value) error {
	fd := getFormatterData(v.Type())
	if fd.err != nil {
		return fd.err
	}
	count, isNull, err := r.ReadObjectHeader()
	if err != nil {
		return err
	}
	if isNull {
		v.SetZero()
		return nil
	}
	if fd.versionTolerant {
		return readVersionTolerant(r, v, fd, count)
	}
	if count > len(fd.fields) {
		return fmt.Errorf("field count %d exceeds %s schema with %d fields", count, v.Type(), len(fd.fields))
	}
	for i, field := range fd.fields {
		value := v.Field(field.index)
		if i >= count {
			continue
		}
		if err = readValue(r, value); err != nil {
			return fmt.Errorf("%s.%s: %w", v.Type(), field.name, err)
		}
	}
	return nil
}

func getFormatterData(t reflect.Type) formatterData {
	formatterCache.mu.RLock()
	cached, found := formatterCache.data[t]
	formatterCache.mu.RUnlock()
	if found {
		return cached
	}
	fd := createFormatterData(t)
	formatterCache.mu.Lock()
	defer formatterCache.mu.Unlock()
	if existing, loaded := formatterCache.data[t]; loaded {
		return existing
	}
	formatterCache.data[t] = fd
	return fd
}

func createFormatterData(t reflect.Type) formatterData {
	fd := formatterData{
		versionTolerant: reflect.PointerTo(t).Implements(reflect.TypeFor[VersionTolerant]()),
	}
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("memorypack")
		if field.PkgPath != "" || tag == "-" {
			continue
		}
		order := i
		if tag != "" {
			parsed, err := strconv.Atoi(tag)
			if err != nil || parsed < 0 {
				fd.err = fmt.Errorf("invalid memorypack order %q on %s.%s", tag, t, field.Name)
				return fd
			}
			order = parsed
		} else if fd.versionTolerant {
			fd.err = fmt.Errorf("version-tolerant field %s.%s requires an explicit memorypack order", t, field.Name)
			return fd
		}
		fd.fields = append(fd.fields, fieldInfo{index: i, name: field.Name, order: order})
	}
	sort.Slice(fd.fields, func(i, j int) bool { return fd.fields[i].order < fd.fields[j].order })
	for i, field := range fd.fields {
		if i > 0 && fd.fields[i-1].order == field.order {
			fd.err = fmt.Errorf("duplicate memorypack order %d on %s", field.order, t)
			return fd
		}
	}
	fd.memberCount = len(fd.fields)
	if fd.versionTolerant && len(fd.fields) > 0 {
		last := fd.fields[len(fd.fields)-1].order
		if last >= int(WideTag)-1 {
			fd.err = fmt.Errorf("version-tolerant order %d exceeds 248", last)
			return fd
		}
		fd.memberCount = last + 1
	}
	if fd.memberCount >= int(WideTag) {
		fd.err = fmt.Errorf("too many fields in %s: %d (max 249)", t, fd.memberCount)
	}
	return fd
}

func writeValue(w *Writer, v reflect.Value) error {
	if err := w.CheckDepth(); err != nil {
		return err
	}
	defer w.EndCheckDepth()
	if !v.IsValid() {
		w.WriteByte(NullObject)
		return nil
	}
	if v.Kind() == reflect.Interface {
		return writeUnion(w, v)
	}
	if marshaler, ok := marshalerFor(v); ok {
		if v.Kind() == reflect.Ptr && v.IsNil() {
			return fmt.Errorf("nil custom value %s requires an explicit nullable representation", v.Type())
		}
		if err := marshaler.MarshalMemoryPack(w); err != nil {
			return fmt.Errorf("custom serialize %s: %w", v.Type(), err)
		}
		return nil
	}
	if scalarSize(v.Kind()) != 0 {
		writeScalar(w, v)
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) || v.Len() > math.MaxInt32 {
			return errors.New("invalid or oversized UTF-8 string")
		}
		w.WriteString(v.String())
		return nil
	case reflect.Struct:
		return serializeStruct(w, v)
	case reflect.Ptr:
		return writePointer(w, v)
	case reflect.Slice, reflect.Array, reflect.Map:
		return writeCollection(w, v)
	default:
		return fmt.Errorf("unsupported type: %s", v.Type())
	}
}

func writePointer(w *Writer, v reflect.Value) error {
	element := v.Type().Elem()
	if scalarSize(element.Kind()) != 0 {
		value := reflect.Zero(element)
		if !v.IsNil() {
			value = v.Elem()
		}
		writeNullable(w, value, !v.IsNil())
		return nil
	}
	if v.IsNil() {
		switch element.Kind() {
		case reflect.String, reflect.Slice, reflect.Map:
			w.WriteNullCollectionHeader()
		default:
			w.WriteByte(NullObject)
		}
		return nil
	}
	return writeValue(w, v.Elem())
}

func writeCollection(w *Writer, v reflect.Value) error {
	if isNilValue(v) {
		w.WriteNullCollectionHeader()
		return nil
	}
	if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[byte]() {
		if v.Len() > math.MaxInt32 {
			return errors.New("byte array exceeds int32 length")
		}
		w.WriteBytes(v.Bytes())
		return nil
	}
	if v.Len() > math.MaxInt32 {
		return fmt.Errorf("collection length %d exceeds int32", v.Len())
	}
	w.WriteCollectionHeader(v.Len())
	if v.Kind() == reflect.Map {
		iter := v.MapRange()
		for iter.Next() {
			if err := writeValue(w, iter.Key()); err != nil {
				return err
			}
			if err := writeValue(w, iter.Value()); err != nil {
				return err
			}
		}
		return nil
	}
	for i := range v.Len() {
		if err := writeValue(w, v.Index(i)); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}
	return nil
}

func readValue(r *Reader, v reflect.Value) error {
	if r.depth >= MaxDepth {
		return fmt.Errorf("deserialization depth exceeded %d", MaxDepth)
	}
	r.depth++
	defer func() { r.depth-- }()
	if v.Kind() == reflect.Interface {
		return readUnion(r, v)
	}
	if unmarshaler, ok := unmarshalerFor(v); ok {
		if err := unmarshaler.UnmarshalMemoryPack(r); err != nil {
			return fmt.Errorf("custom deserialize %s: %w", v.Type(), err)
		}
		return nil
	}
	if scalarSize(v.Kind()) != 0 {
		return readScalar(r, v)
	}
	switch v.Kind() {
	case reflect.String:
		value, err := r.ReadString()
		if err == nil {
			v.SetString(value)
		}
		return err
	case reflect.Struct:
		return deserializeStruct(r, v)
	case reflect.Ptr:
		return readPointer(r, v)
	case reflect.Slice, reflect.Array, reflect.Map:
		return readCollection(r, v)
	default:
		return fmt.Errorf("unsupported type: %s", v.Type())
	}
}

func readPointer(r *Reader, v reflect.Value) error {
	if v.Type().Implements(reflect.TypeFor[Unmarshaler]()) {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return readValue(r, v.Elem())
	}
	element := v.Type().Elem()
	if scalarSize(element.Kind()) != 0 {
		value := reflect.New(element)
		present, err := readNullable(r, value.Elem())
		if err != nil {
			return err
		}
		if present {
			v.Set(value)
		} else {
			v.SetZero()
		}
		return nil
	}
	var width int
	switch element.Kind() {
	case reflect.String, reflect.Slice, reflect.Map:
		width = int32Size
	default:
		width = 1
	}
	marker, err := r.Peek(width)
	if err != nil {
		return err
	}
	isNull := true
	for _, b := range marker {
		isNull = isNull && b == NullObject
	}
	if isNull {
		r.pos += width
		v.SetZero()
		return nil
	}
	if v.IsNil() {
		v.Set(reflect.New(element))
	}
	return readValue(r, v.Elem())
}

func readCollection(r *Reader, v reflect.Value) error {
	if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[byte]() {
		data, err := r.ReadBytes()
		if err == nil {
			v.SetBytes(data)
		}
		return err
	}
	length, isNull, err := r.ReadCollectionHeader()
	if err != nil {
		return err
	}
	if isNull {
		v.SetZero()
		return nil
	}
	if v.Kind() == reflect.Map {
		return readMap(r, v, length)
	}
	if v.Kind() == reflect.Array {
		if length != v.Len() {
			return fmt.Errorf("array length %d does not match %s", length, v.Type())
		}
		for i := range length {
			if err = readValue(r, v.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}
	// Grow only after a value has been decoded; an untrusted count must not
	// allocate the complete destination before its payload has been checked.
	result := reflect.MakeSlice(v.Type(), 0, 0)
	for i := range length {
		element := reflect.New(v.Type().Elem()).Elem()
		if err = readValue(r, element); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
		result = reflect.Append(result, element)
	}
	v.Set(result)
	return nil
}

func readMap(r *Reader, v reflect.Value, length int) error {
	result := reflect.MakeMap(v.Type())
	for range length {
		key := reflect.New(v.Type().Key()).Elem()
		value := reflect.New(v.Type().Elem()).Elem()
		if err := readValue(r, key); err != nil {
			return err
		}
		if !key.Comparable() {
			return errors.New("decoded map key is not comparable")
		}
		if result.MapIndex(key).IsValid() {
			return errors.New("duplicate map key")
		}
		if err := readValue(r, value); err != nil {
			return err
		}
		result.SetMapIndex(key, value)
	}
	v.Set(result)
	return nil
}
