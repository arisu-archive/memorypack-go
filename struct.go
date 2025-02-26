package memorypack

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var formatterCache sync.Map // map[reflect.Type]formatterData

type formatterData struct {
	fields []fieldInfo
}

type fieldInfo struct {
	index int
	kind  reflect.Kind
	name  string
	order int
}

type Formatter interface {
	Serialize(writer *Writer) error
	Deserialize(reader *Reader) error
}

// SerializeStruct serializes a struct to the writer.
func SerializeStruct(writer *Writer, value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("SerializeStruct only accepts struct values")
	}

	t := v.Type()
	fd := getFormatterData(t)

	// Write object header with field count
	if err := writer.WriteObjectHeader(len(fd.fields)); err != nil {
		return err
	}

	// Write each field
	for _, field := range fd.fields {
		fieldValue := v.Field(field.index)
		if err := writeValue(writer, fieldValue); err != nil {
			return err
		}
	}

	return nil
}

// DeserializeStruct deserializes a struct from the reader.
func DeserializeStruct(reader *Reader, value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("DeserializeStruct requires a pointer to a struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("DeserializeStruct requires a pointer to a struct")
	}

	t := v.Type()
	fd := getFormatterData(t)

	// Read object header
	fieldCount, isNull, err := reader.ReadObjectHeader()
	if err != nil {
		return err
	}

	if isNull {
		// Cannot set struct to null, just return
		return nil
	}

	// Verify field count matches
	if fieldCount != len(fd.fields) {
		return fmt.Errorf("field count mismatch during deserialization")
	}

	// Read each field
	for _, field := range fd.fields {
		fieldValue := v.Field(field.index)
		if fieldValue.CanSet() {
			if err = readValue(reader, fieldValue); err != nil {
				return err
			}
		} else {
			// Skip over this field in the data
			if err = skipValue(reader, field.kind); err != nil {
				return err
			}
		}
	}

	return nil
}

// getFormatterData gets or creates formatter data for a type.
func getFormatterData(t reflect.Type) formatterData {
	if cachedData, found := formatterCache.Load(t); found {
		return cachedData.(formatterData)
	}

	fd := createFormatterData(t)
	formatterCache.Store(t, fd)
	return fd
}

// createFormatterData creates formatter data for a type.
func createFormatterData(t reflect.Type) formatterData {
	fd := formatterData{
		fields: make([]fieldInfo, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			// Skip unexported fields
			continue
		}

		// Check tag for order
		order := i
		tag := field.Tag.Get("memorypack")
		if tag != "" && tag != "-" {
			parts := strings.Split(tag, ",")
			if orderStr := parts[0]; orderStr != "" {
				if parsedOrder, err := strconv.Atoi(orderStr); err == nil {
					order = parsedOrder
				}
			}
		}

		// Skip fields that are not tagged or tagged with '-'
		if tag == "-" {
			continue
		}

		fd.fields = append(fd.fields, fieldInfo{
			index: i,
			kind:  field.Type.Kind(),
			name:  field.Name,
			order: order,
		})
	}

	// Sort fields by the specified order
	sort.Slice(fd.fields, func(i, j int) bool {
		return fd.fields[i].order < fd.fields[j].order
	})

	return fd
}

// writeValue handles writing any reflected value.
func writeValue(writer *Writer, v reflect.Value) error {
	if err := writer.CheckDepth(); err != nil {
		return err
	}
	defer writer.EndCheckDepth()
	switch v.Kind() {
	case reflect.Bool:
		writer.WriteBool(v.Bool())
	case reflect.Int8:
		writer.WriteByte(byte(v.Int()))
	case reflect.Int16:
		writer.WriteInt16(int16(v.Int()))
	case reflect.Int32:
		writer.WriteInt32(int32(v.Int()))
	case reflect.Int, reflect.Int64:
		writer.WriteInt64(v.Int())
	case reflect.Float32:
		writer.WriteFloat32(float32(v.Float()))
	case reflect.Float64:
		writer.WriteFloat64(v.Float())
	case reflect.String:
		writer.WriteString(v.String())
	case reflect.Slice:
		if v.IsNil() {
			writer.WriteNullCollectionHeader()
			return nil
		}

		if v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte has special treatment
			writer.WriteBytes(v.Bytes())
		} else {
			// Other slices
			writer.WriteCollectionHeader(v.Len())
			for i := 0; i < v.Len(); i++ {
				if err := writeValue(writer, v.Index(i)); err != nil {
					return err
				}
			}
		}
	case reflect.Array:
		length := v.Len()
		writer.WriteCollectionHeader(length)
		for i := range length {
			if err := writeValue(writer, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		if v.IsNil() {
			writer.WriteNullCollectionHeader()
			return nil
		}

		writer.WriteCollectionHeader(v.Len())
		if v.Len() > 0 {
			iter := v.MapRange()
			for iter.Next() {
				if err := writeValue(writer, iter.Key()); err != nil {
					return err
				}
				if err := writeValue(writer, iter.Value()); err != nil {
					return err
				}
			}
		}
	case reflect.Struct:
		return SerializeStruct(writer, v.Interface())
	case reflect.Ptr:
		if !v.IsNil() {
			return writeValue(writer, v.Elem())
		}
		writer.WriteByte(NullObject)
	default:
		return fmt.Errorf("unsupported type: %s", v.Kind())
	}
	return nil
}

// readValue handles reading any reflected value.
func readValue(reader *Reader, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Bool:
		val, err := reader.ReadBool()
		if err != nil {
			return err
		}
		v.SetBool(val)
	case reflect.Int8:
		val, err := reader.ReadByte()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int16:
		val, err := reader.ReadInt16()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int32:
		val, err := reader.ReadInt32()
		if err != nil {
			return err
		}
		v.SetInt(int64(val))
	case reflect.Int, reflect.Int64:
		val, err := reader.ReadInt64()
		if err != nil {
			return err
		}
		v.SetInt(val)
	case reflect.Float32:
		val, err := reader.ReadFloat32()
		if err != nil {
			return err
		}
		v.SetFloat(float64(val))
	case reflect.Float64:
		val, err := reader.ReadFloat64()
		if err != nil {
			return err
		}
		v.SetFloat(val)
	case reflect.String:
		val, err := reader.ReadString()
		if err != nil {
			return err
		}
		v.SetString(val)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte has special treatment
			bytes, err := reader.ReadBytes()
			if err != nil {
				return err
			}
			v.SetBytes(bytes)
		} else {
			// Other slices
			length, isNull, err := reader.ReadCollectionHeader()
			if err != nil {
				return err
			}
			if isNull {
				v.Set(reflect.Zero(v.Type()))
				return nil
			}

			slice := reflect.MakeSlice(v.Type(), length, length)
			for i := range length {
				if err = readValue(reader, slice.Index(i)); err != nil {
					return err
				}
			}
			v.Set(slice)
		}
	case reflect.Array:
		length, isNull, err := reader.ReadCollectionHeader()
		if err != nil {
			return err
		}
		if isNull {
			// Can't set nil to array, so skip
			return nil
		}

		for i := range length {
			if err = readValue(reader, v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		length, isNull, err := reader.ReadCollectionHeader()
		if err != nil {
			return err
		}
		if isNull {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}

		mapType := v.Type()
		mapValue := reflect.MakeMapWithSize(mapType, length)

		for range length {
			keyType := mapType.Key()
			valueType := mapType.Elem()

			key := reflect.New(keyType).Elem()
			value := reflect.New(valueType).Elem()

			if err = readValue(reader, key); err != nil {
				return err
			}
			if err = readValue(reader, value); err != nil {
				return err
			}

			mapValue.SetMapIndex(key, value)
		}

		v.Set(mapValue)
	case reflect.Struct:
		return DeserializeStruct(reader, v.Addr().Interface())
	case reflect.Ptr:
		b, err := reader.Peek(1)
		if err != nil {
			return err
		}
		if b[0] == NullObject {
			// Consume the null marker
			if _, err = reader.ReadByte(); err != nil {
				return err
			}
			// Set to nil
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		// Object with members
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return readValue(reader, v.Elem())
	default:
		return fmt.Errorf("unsupported type: %s", v.Kind())
	}
	return nil
}

// skipValue skips over a value in the reader.
func skipValue(reader *Reader, kind reflect.Kind) error {
	switch kind {
	case reflect.Bool, reflect.Int8, reflect.Uint8:
		_, err := reader.ReadByte()
		return err
	case reflect.Int16, reflect.Uint16:
		_, err := reader.ReadInt16()
		return err
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		_, err := reader.ReadInt32()
		return err
	case reflect.Int64, reflect.Uint64, reflect.Float64:
		_, err := reader.ReadInt64()
		return err
	case reflect.String:
		_, err := reader.ReadString()
		return err
	case reflect.Slice, reflect.Array:
		length, isNull, err := reader.ReadCollectionHeader()
		if err != nil {
			return err
		}
		if !isNull {
			for range length {
				// Assuming int32 elements for simple skipping
				if _, err = reader.ReadInt32(); err != nil {
					return err
				}
			}
		}
		return nil
	case reflect.Map:
		length, isNull, err := reader.ReadCollectionHeader()
		if err != nil {
			return err
		}
		if !isNull {
			for range length {
				// Skip key and value (assuming strings for simplicity)
				if _, err = reader.ReadString(); err != nil {
					return err
				}
				_, err = reader.ReadString()
				if err != nil {
					return err
				}
			}
		}
		return nil
	case reflect.Struct:
		// Skip object header
		_, isNull, err := reader.ReadObjectHeader()
		if err != nil {
			return err
		}
		if !isNull {
			return fmt.Errorf("skipping struct fields not fully implemented")
		}
		return nil
	case reflect.Ptr:
		header, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if header != NullObject {
			return fmt.Errorf("skipping pointer values not fully implemented")
		}
		return nil
	default:
		return fmt.Errorf("skipping unsupported type: %s", kind)
	}
}
