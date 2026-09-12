package memorypack

import (
	"fmt"
	"reflect"
	"sync"
)

// UnionMember associates a wire tag with a concrete implementation. Value is
// used only for its type; a typed nil such as (*Message)(nil) is sufficient.
type UnionMember struct {
	Tag   uint16
	Value any
}

type unionData struct {
	byTag  map[uint16]reflect.Type
	byType map[reflect.Type]uint16
}

//nolint:gochecknoglobals // RegisterUnion defines a process-wide schema; the mutex publishes immutable entries.
var unionRegistry = struct {
	mu   sync.RWMutex
	data map[reflect.Type]unionData
}{data: make(map[reflect.Type]unionData)}

// RegisterUnion registers the complete schema for interface T. Tags and types
// must be unique, and every member must implement T. A schema cannot be replaced;
// registering the identical schema again is harmless. Registration and lookups
// are safe to call concurrently. Register before encoding or decoding T.
func RegisterUnion[T any](members ...UnionMember) error {
	target := reflect.TypeFor[T]()
	if target.Kind() != reflect.Interface {
		return fmt.Errorf("union target must be an interface: %s", target)
	}
	if len(members) == 0 {
		return fmt.Errorf("union %s requires at least one member", target)
	}
	data := unionData{byTag: make(map[uint16]reflect.Type), byType: make(map[reflect.Type]uint16)}
	for _, member := range members {
		typ := reflect.TypeOf(member.Value)
		if typ == nil || typ.Kind() == reflect.Interface || !typ.Implements(target) {
			return fmt.Errorf("union member %v does not implement %s", typ, target)
		}
		if _, exists := data.byTag[member.Tag]; exists {
			return fmt.Errorf("duplicate union tag: %d", member.Tag)
		}
		if _, exists := data.byType[typ]; exists {
			return fmt.Errorf("duplicate union type: %s", typ)
		}
		data.byTag[member.Tag], data.byType[typ] = typ, member.Tag
	}
	unionRegistry.mu.Lock()
	defer unionRegistry.mu.Unlock()
	previous, loaded := unionRegistry.data[target]
	if loaded && !reflect.DeepEqual(previous.byTag, data.byTag) {
		return fmt.Errorf("union %s already has a different schema", target)
	}
	if !loaded {
		unionRegistry.data[target] = data
	}
	return nil
}

func registeredUnion(t reflect.Type) (unionData, bool) {
	unionRegistry.mu.RLock()
	defer unionRegistry.mu.RUnlock()
	data, found := unionRegistry.data[t]
	return data, found
}

func writeUnion(w *Writer, v reflect.Value) error {
	registered, ok := registeredUnion(v.Type())
	if !ok {
		return fmt.Errorf("unregistered union interface: %s", v.Type())
	}
	if v.IsNil() {
		w.WriteNullUnionHeader()
		return nil
	}
	value := v.Elem()
	tag, ok := registered.byType[value.Type()]
	if !ok {
		return fmt.Errorf("unregistered union member %s for %s", value.Type(), v.Type())
	}
	if isNilValue(value) {
		w.WriteNullUnionHeader()
		return nil
	}
	w.WriteUnionHeader(tag)
	return writeValue(w, value)
}

func readUnion(r *Reader, v reflect.Value) error {
	registered, ok := registeredUnion(v.Type())
	if !ok {
		return fmt.Errorf("unregistered union interface: %s", v.Type())
	}
	tag, isNull, err := r.ReadUnionHeader()
	if err != nil {
		return err
	}
	if isNull {
		v.SetZero()
		return nil
	}
	typ, ok := registered.byTag[tag]
	if !ok {
		return fmt.Errorf("unknown union tag %d for %s", tag, v.Type())
	}
	value := reflect.New(typ).Elem()
	if !v.IsNil() && v.Elem().Type() == typ {
		value.Set(v.Elem())
	}
	if err = readValue(r, value); err != nil {
		return fmt.Errorf("union tag %d: %w", tag, err)
	}
	if isNilValue(value) {
		v.SetZero()
	} else {
		v.Set(value)
	}
	return nil
}

func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}
