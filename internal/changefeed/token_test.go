package changefeed

import (
	"strings"
	"testing"
)

func TestSealer_RoundTrip(t *testing.T) {
	s, err := newSealer()
	if err != nil {
		t.Fatal(err)
	}
	for _, seq := range []uint64{0, 1, 4096, 1<<64 - 1} {
		tok := s.seal(seq)
		if len(tok) != tokenLen {
			t.Fatalf("len(token) = %d, want %d", len(tok), tokenLen)
		}
		got, ok := s.open(tok)
		if !ok || got != seq {
			t.Fatalf("open(seal(%d)) = %d, %v", seq, got, ok)
		}
	}
	if a, b := s.seal(7), s.seal(7); a == b {
		t.Fatal("two tokens for one seq are equal; the nonce is not random")
	}
}

func TestSealer_RejectsForeignAndMalformed(t *testing.T) {
	s, _ := newSealer()
	other, _ := newSealer()
	good := s.seal(42)

	flip := []byte(good)
	if flip[10] == 'A' {
		flip[10] = 'B'
	} else {
		flip[10] = 'A'
	}

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"foreign key", other.seal(42)},
		{"flipped character", string(flip)},
		{"truncated", good[:len(good)-1]},
		{"oversize", good + "A"},
		{"padding character", good[:len(good)-1] + "="},
		{"standard alphabet", strings.NewReplacer("-", "+", "_", "/").Replace(good)},
		{"not base64", strings.Repeat("!", tokenLen)},
		{"null bytes", strings.Repeat("\x00", tokenLen)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "standard alphabet" && tc.token == good {
				t.Skip("token has no URL-specific characters")
			}
			if _, ok := s.open(tc.token); ok {
				t.Fatalf("open(%q) accepted", tc.token)
			}
		})
	}
}
