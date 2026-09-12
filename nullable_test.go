package memorypack_test

import (
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

func TestNullableCSharpFixtures(t *testing.T) {
	checkCSharpFixture(t, "nullable-bool", memorypack.Nullable[bool]{Value: true, HasValue: true})
	checkCSharpFixture(t, "nullable-byte", memorypack.Nullable[uint8]{Value: 255, HasValue: true})
	checkCSharpFixture(t, "nullable-short", memorypack.Nullable[int16]{Value: -255, HasValue: true})
	checkCSharpFixture(t, "nullable-int", memorypack.Nullable[int32]{Value: 255, HasValue: true})
	checkCSharpFixture(t, "nullable-uint", memorypack.Nullable[uint32]{Value: 4294967295, HasValue: true})
	checkCSharpFixture(t, "nullable-long", memorypack.Nullable[int64]{Value: -1, HasValue: true})
	checkCSharpFixture(t, "nullable-ulong", memorypack.Nullable[uint64]{Value: 18446744073709551615, HasValue: true})
	checkCSharpFixture(t, "nullable-float", memorypack.Nullable[float32]{Value: 1.5, HasValue: true})
	checkCSharpFixture(t, "nullable-double", memorypack.Nullable[float64]{Value: -2.25, HasValue: true})
	checkCSharpFixture(t, "nullable-int-null", memorypack.Nullable[int32]{})
	checkCSharpFixture(t, "nullable-long-null", memorypack.Nullable[int64]{})
	number := int32(255)
	checkCSharpFixture(t, "nullable-int", &number)
	checkCSharpFixture(t, "nullable-int-null", (*int32)(nil))
	checkCSharpFixture(t, "nullable-holder", struct {
		Number *int32
		Text   *string
	}{Number: &number})
}

func TestNullableBoundaries(t *testing.T) {
	valid := csharpFixture(t, "nullable-long")
	value := memorypack.Nullable[int64]{Value: 42, HasValue: true}
	for length := range valid {
		if err := memorypack.Deserialize(valid[:length], &value); err == nil {
			t.Errorf("accepted nullable truncated at %d", length)
		}
		if value.Value != 42 || !value.HasValue {
			t.Fatal("failed nullable decode changed destination")
		}
	}
	invalid := append([]byte(nil), valid...)
	invalid[0] = 2
	if err := memorypack.Deserialize(invalid, &value); err == nil {
		t.Fatal("accepted invalid nullable presence flag")
	}
	if err := memorypack.Deserialize(csharpFixture(t, "nullable-long-null"), &value); err != nil {
		t.Fatal(err)
	}
	if value.HasValue || value.Value != 0 {
		t.Fatalf("null did not clear destination: %+v", value)
	}
	var pointer *int64
	if err := memorypack.Deserialize(valid, &pointer); err != nil || pointer == nil || *pointer != -1 {
		t.Fatalf("nullable value beginning with FF: pointer %v, error %v", pointer, err)
	}
}
