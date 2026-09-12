package memorypack_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/arisu-archive/memorypack-go"
)

func csharpFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]string
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	encoded, ok := fixtures[name]
	if !ok {
		t.Fatalf("missing C# fixture %q", name)
	}
	data, err = hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func checkCSharpFixture[T any](t *testing.T, name string, value T) {
	t.Helper()
	expected := csharpFixture(t, name)
	actual, err := memorypack.Serialize(&value)
	if err != nil {
		t.Errorf("%s encode: %v", name, err)
	} else if !bytes.Equal(actual, expected) {
		t.Errorf("%s bytes: got %X, want C# %X", name, actual, expected)
	}
	var decoded T
	if err = memorypack.Deserialize(expected, &decoded); err != nil {
		t.Errorf("%s decode: %v", name, err)
	} else if !reflect.DeepEqual(decoded, value) {
		t.Errorf("%s value: got %#v, want %#v", name, decoded, value)
	}
}

func TestCSharpPrimitiveFixtures(t *testing.T) {
	checkCSharpFixture(t, "int8-negative", int8(-128))
	checkCSharpFixture(t, "uint8", uint8(255))
	checkCSharpFixture(t, "uint16", uint16(65535))
	checkCSharpFixture(t, "uint32", uint32(4294967295))
	checkCSharpFixture(t, "uint64", uint64(18446744073709551615))
	checkCSharpFixture(t, "utf8", "A界\U0001D11E")
	checkCSharpFixture(t, "string-empty", "")
	checkCSharpFixture(t, "string-null", (*string)(nil))
}

func TestCSharpUTF16Decode(t *testing.T) {
	var decoded string
	if err := memorypack.Deserialize(csharpFixture(t, "utf16"), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != "A界\U0001D11E" {
		t.Fatalf("UTF-16 decoded %q, want %q", decoded, "A界\U0001D11E")
	}
}
