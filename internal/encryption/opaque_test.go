package encryption

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOpaque_NewDefensiveCopy(t *testing.T) {
	src := []byte{1, 2, 3, 4}
	o := NewOpaque(src)
	// Mutating the source must not affect the Opaque.
	src[0] = 0xFF
	got := o.Bytes()
	want := []byte{1, 2, 3, 4}
	if !bytes.Equal(got, want) {
		t.Fatalf("Bytes() = %v, want %v (source mutation leaked)", got, want)
	}
}

func TestOpaque_BytesReturnsCopy(t *testing.T) {
	o := NewOpaque([]byte{1, 2, 3})
	b1 := o.Bytes()
	b1[0] = 0xFF
	b2 := o.Bytes()
	if b2[0] == 0xFF {
		t.Fatalf("Bytes() returned aliased slice; mutation leaked")
	}
}

func TestOpaque_StringRedacts(t *testing.T) {
	o := NewOpaque([]byte("SECRET-PLAINTEXT-XYZ"))
	if got := o.String(); got != "<encrypted>" {
		t.Errorf("String() = %q, want %q", got, "<encrypted>")
	}
	// Also via a Stringer-consuming format verb: gocritic would
	// flag fmt.Sprint(o) as "use o.String() instead", but the
	// whole point is to verify fmt verbs go through String.
	if got := fmt.Sprintf("%s", any(o)); got != "<encrypted>" {
		t.Errorf("fmt.Sprintf(%%s) = %q, want %q", got, "<encrypted>")
	}
}

func TestOpaque_MarshalJSON(t *testing.T) {
	// json.Marshal HTML-escapes `<` and `>` by default, so the
	// wire-format string is `"\u003cencrypted\u003e"`. Both forms
	// unmarshal to the same Go string "<encrypted>" — that's the
	// invariant we care about.
	const wantUnescaped = "<encrypted>"

	o := NewOpaque([]byte("SECRET-PLAINTEXT-XYZ"))
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if umErr := json.Unmarshal(b, &got); umErr != nil {
		t.Fatalf("round-trip unmarshal: %v", umErr)
	}
	if got != wantUnescaped {
		t.Fatalf("MarshalJSON round-trip = %q, want %q", got, wantUnescaped)
	}

	// Marshaling a struct with an Opaque field should also redact.
	wrap := struct {
		Value Opaque `json:"value"`
	}{Value: o}
	b, err = json.Marshal(wrap)
	if err != nil {
		t.Fatal(err)
	}
	var gotStruct struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(b, &gotStruct); err != nil {
		t.Fatalf("wrapped round-trip: %v", err)
	}
	if gotStruct.Value != wantUnescaped {
		t.Fatalf("wrapped value = %q, want %q", gotStruct.Value, wantUnescaped)
	}
}

func TestOpaque_Len(t *testing.T) {
	o := NewOpaque([]byte{1, 2, 3, 4, 5})
	if o.Len() != 5 {
		t.Errorf("Len() = %d, want 5", o.Len())
	}
	empty := NewOpaque(nil)
	if empty.Len() != 0 {
		t.Errorf("Len() of empty = %d, want 0", empty.Len())
	}
}

// TestOpaque_NoLeakOnFmtVerbs is a belt-and-braces check that no
// format verb leaks the raw ciphertext bytes. If someone adds a
// String-like method in future that doesn't redact, this test fails.
func TestOpaque_NoLeakOnFmtVerbs(t *testing.T) {
	const marker = "LEAK-CANARY-BYTES-ZZZ"
	o := NewOpaque([]byte(marker))
	for _, verb := range []string{"%v", "%s", "%q"} {
		out := fmt.Sprintf(verb, o)
		if strings.Contains(out, marker) {
			t.Errorf("verb %s leaked marker: %q", verb, out)
		}
	}
}

// TestOpaque_ValueCopyPreservesBytes documents the contract with
// entity.CloneValue: Opaque is a value type with a private slice; a
// shallow value-copy (return v) keeps the bytes reachable without
// cloning. Since Opaque has no public mutator, the aliased slice
// stays immutable through the public API.
//
// This test is here (not in entity) so internal/entity doesn't need
// a dependency on internal/encryption just to exercise the case.
func TestOpaque_ValueCopyPreservesBytes(t *testing.T) {
	original := NewOpaque([]byte{10, 20, 30, 40})

	// Simulate what entity.CloneValue's default branch does: return v.
	var cloned any = original

	o2, ok := cloned.(Opaque)
	if !ok {
		t.Fatal("type assertion back to Opaque failed after value-copy")
	}
	if !bytes.Equal(o2.Bytes(), []byte{10, 20, 30, 40}) {
		t.Fatalf("value-copied Opaque has different Bytes(): %v", o2.Bytes())
	}
	if o2.String() != "<encrypted>" {
		t.Errorf("value-copied Opaque no longer redacts: %q", o2.String())
	}
}
