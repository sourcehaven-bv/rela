package encryption

import (
	"bytes"
	"errors"
	"testing"
)

func TestHeader_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		h    Header
		body []byte
	}{
		{"simple", Header{Version: 1, Path: "entities/tickets/TKT-1.md"}, []byte("---\nid: TKT-1\n---\nhello\n")},
		{"large version", Header{Version: 12345, Path: "relations/A--r--B.md"}, []byte("body")},
		{"empty body", Header{Version: 7, Path: "x.md"}, []byte{}},
		{"binary body", Header{Version: 1, Path: "attachments/ab/ab3f.png"}, []byte{0x00, 0x01, 0xFF, 0xFE, '\n', '-', '-', '-', '\n'}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := tc.h.Encode()
			// Concatenate encoded header + body → parse round-trips.
			blob := make([]byte, 0, len(encoded)+len(tc.body))
			blob = append(blob, encoded...)
			blob = append(blob, tc.body...)
			got, body, err := ParseHeader(blob)
			if err != nil {
				t.Fatalf("ParseHeader: %v", err)
			}
			if got.Version != tc.h.Version || got.Path != tc.h.Path {
				t.Errorf("got %+v, want %+v", got, tc.h)
			}
			if !bytes.Equal(body, tc.body) {
				t.Errorf("body mismatch: got %q, want %q", body, tc.body)
			}
		})
	}
}

func TestParseHeader_FieldOrderIsTolerant(t *testing.T) {
	// path=X v=Y should parse the same as v=Y path=X.
	blob := []byte("rela path=x.md v=5\nbody")
	h, body, err := ParseHeader(blob)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if h.Version != 5 || h.Path != "x.md" {
		t.Errorf("got %+v, want v=5 path=x.md", h)
	}
	if string(body) != "body" {
		t.Errorf("body = %q", body)
	}
}

func TestParseHeader_Malformed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"no terminator", "rela v=1 path=x.md"},
		{"bad magic", "nope v=1 path=x.md\nbody"},
		{"missing v", "rela path=x.md\nbody"},
		{"missing path", "rela v=1\nbody"},
		{"bad version", "rela v=abc path=x.md\nbody"},
		{"unknown field", "rela v=1 path=x.md whatever=yes\nbody"},
		{"bare token", "rela v=1 path=x.md oops\nbody"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseHeader([]byte(tc.input))
			if !errors.Is(err, ErrMalformedHeader) {
				t.Errorf("expected ErrMalformedHeader, got %v", err)
			}
		})
	}
}

func TestHeader_BodyPreservedExactly(t *testing.T) {
	// An entity body that begins with --- (YAML frontmatter) must
	// round-trip byte-for-byte even though the header format also
	// uses ASCII lines.
	body := []byte("---\nid: REQ-1\ntype: requirement\n---\n\nSome body with\n---\nembedded divider\n")
	h := Header{Version: 42, Path: "entities/requirements/REQ-1.md"}
	enc := h.Encode()
	blob := make([]byte, 0, len(enc)+len(body))
	blob = append(blob, enc...)
	blob = append(blob, body...)

	_, got, err := ParseHeader(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("body not preserved: got %q, want %q", got, body)
	}
}

func TestHeader_OverheadIsSmall(t *testing.T) {
	// The header should be compact — much less than a multi-line
	// YAML equivalent would be. Sanity check for the key=value
	// design choice.
	h := Header{Version: 7, Path: "entities/requirements/REQ-001.md"}
	encoded := h.Encode()
	if len(encoded) > 80 {
		t.Errorf("header overhead = %d bytes, want ≤ 80", len(encoded))
	}
	// Must end in the terminator so the parser can find the split.
	if encoded[len(encoded)-1] != '\n' {
		t.Errorf("header must end in \\n, got %q", encoded[len(encoded)-1])
	}
}
