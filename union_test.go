package memorypack_test

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

type (
	message         interface{ isMessage() }
	numberMessage   struct{ Value int32 }
	boundaryMessage struct{ Value int32 }
	textMessage     struct{ Value string }
	maxMessage      struct{ Value uint16 }
	unknownMessage  struct{}
)

func (*numberMessage) isMessage()   {}
func (*boundaryMessage) isMessage() {}
func (*textMessage) isMessage()     {}
func (*maxMessage) isMessage()      {}
func (*unknownMessage) isMessage()  {}

func registerMessages() error {
	return memorypack.RegisterUnion[message](
		memorypack.UnionMember{Tag: 0, Value: (*numberMessage)(nil)},
		memorypack.UnionMember{Tag: 249, Value: (*boundaryMessage)(nil)},
		memorypack.UnionMember{Tag: 250, Value: (*textMessage)(nil)},
		memorypack.UnionMember{Tag: 65535, Value: (*maxMessage)(nil)},
	)
}

func TestUnionCSharpFixtures(t *testing.T) {
	if err := registerMessages(); err != nil {
		t.Fatal(err)
	}
	checkCSharpFixture[message](t, "union-small", &numberMessage{Value: 42})
	checkCSharpFixture[message](t, "union-249", &boundaryMessage{Value: -1})
	checkCSharpFixture[message](t, "union-wide", &textMessage{Value: "hello"})
	checkCSharpFixture[message](t, "union-max", &maxMessage{Value: 65535})
	checkCSharpFixture[message](t, "union-null", nil)
	checkCSharpFixture(t, "union-list", []message{&numberMessage{Value: 42}, nil, &textMessage{Value: "hello"}})
	checkCSharpFixture(t, "union-holder", struct{ Message message }{Message: &textMessage{Value: "hello"}})
	checkCSharpFixture(t, "union-map", map[string]message{"item": &numberMessage{Value: 42}})

	var typedNil message = (*numberMessage)(nil)
	data, err := memorypack.Serialize(&typedNil)
	if err != nil || !bytes.Equal(data, []byte{255}) {
		t.Fatalf("typed nil union: bytes %X, error %v", data, err)
	}
	if err = memorypack.Deserialize(data, &typedNil); err != nil || typedNil != nil {
		t.Fatalf("null union must clear existing value: got %v, error %v", typedNil, err)
	}
	existing := &numberMessage{Value: 42}
	var reused message = existing
	if err = memorypack.Deserialize([]byte{0, 0}, &reused); err != nil || reused != existing || existing.Value != 42 {
		t.Fatalf("union overwrite lost identity or missing field: %v, %v", reused, err)
	}
}

func TestUnionHeaders(t *testing.T) {
	cases := []struct {
		tag  uint16
		wire []byte
	}{
		{0, []byte{0}},
		{249, []byte{249}},
		{250, []byte{250, 250, 0}},
		{251, []byte{250, 251, 0}},
		{255, []byte{250, 255, 0}},
		{256, []byte{250, 0, 1}},
		{65535, []byte{250, 255, 255}},
	}
	for _, tc := range cases {
		t.Run(strconv.FormatUint(uint64(tc.tag), 10), func(t *testing.T) {
			w := memorypack.NewWriter(1)
			w.WriteUnionHeader(tc.tag)
			if !bytes.Equal(w.GetBytes(), tc.wire) {
				t.Fatalf("tag bytes: got %X, want %X", w.GetBytes(), tc.wire)
			}
			r := memorypack.NewReader(tc.wire)
			tag, null, err := r.ReadUnionHeader()
			if err != nil || null || tag != tc.tag || r.Remaining() != 0 {
				t.Fatalf("header: tag %d, null %v, error %v, remaining %d", tag, null, err, r.Remaining())
			}
		})
	}
	for _, data := range [][]byte{nil, {250}, {250, 1}, {251}, {252}, {253}, {254}} {
		if _, _, err := memorypack.NewReader(data).ReadUnionHeader(); err == nil {
			t.Errorf("accepted invalid union header %X", data)
		}
	}
}

func TestUnionValidation(t *testing.T) {
	type registrationMessage interface{ message }
	invalid := [][]memorypack.UnionMember{
		nil,
		{{Tag: 0, Value: nil}},
		{{Tag: 0, Value: "wrong type"}},
		{{Tag: 0, Value: (*numberMessage)(nil)}, {Tag: 0, Value: (*textMessage)(nil)}},
		{{Tag: 0, Value: (*numberMessage)(nil)}, {Tag: 1, Value: (*numberMessage)(nil)}},
	}
	for _, members := range invalid {
		if err := memorypack.RegisterUnion[registrationMessage](members...); err == nil {
			t.Errorf("accepted invalid registration: %#v", members)
		}
	}
	var absent registrationMessage
	if _, err := memorypack.Serialize(&absent); err == nil {
		t.Fatal("failed registrations must not publish a schema")
	}
	if err := memorypack.RegisterUnion[numberMessage](memorypack.UnionMember{Value: numberMessage{}}); err == nil {
		t.Fatal("accepted a non-interface union target")
	}
	if err := registerMessages(); err != nil {
		t.Fatal(err)
	}
	replacement := memorypack.UnionMember{Tag: 9, Value: (*numberMessage)(nil)}
	if err := memorypack.RegisterUnion[message](replacement); err == nil {
		t.Fatal("replaced an existing union schema")
	}
	var unknown message = &unknownMessage{}
	if _, err := memorypack.Serialize(&unknown); err == nil {
		t.Fatal("serialized an unregistered implementation")
	}
	var decoded message
	if err := memorypack.Deserialize([]byte{1, 0}, &decoded); err == nil {
		t.Fatal("accepted an unknown union tag")
	}
	valid := csharpFixture(t, "union-wide")
	for length := range valid {
		if err := memorypack.Deserialize(valid[:length], &decoded); err == nil {
			t.Errorf("accepted union truncated at byte %d", length)
		}
	}
}

func TestUnionConcurrentUse(t *testing.T) {
	expected := csharpFixture(t, "union-small")
	start := make(chan struct{})
	errors := make(chan error, 16)
	var workers sync.WaitGroup
	for range cap(errors) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			if err := registerMessages(); err != nil {
				errors <- err
				return
			}
			var value message = &numberMessage{Value: 42}
			data, err := memorypack.Serialize(&value)
			if err != nil {
				errors <- fmt.Errorf("serialize union: %w", err)
				return
			}
			if !bytes.Equal(data, expected) {
				errors <- fmt.Errorf("union bytes %X, want %X", data, expected)
				return
			}
			var decoded message
			if err = memorypack.Deserialize(expected, &decoded); err != nil {
				errors <- err
				return
			}
			if number, ok := decoded.(*numberMessage); !ok || number.Value != 42 {
				errors <- fmt.Errorf("decoded unexpected union value: %#v", decoded)
			}
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
