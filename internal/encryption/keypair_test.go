package encryption

import (
	"bytes"
	"errors"
	"testing"
)

func TestGenerateKeypair_Unique(t *testing.T) {
	const n = 10
	seen := make(map[string]struct{})
	for i := range n {
		k := mustGenerate(t)
		b := k.x25519.PublicKey().Bytes()
		key := string(b)
		if _, dup := seen[key]; dup {
			t.Fatalf("duplicate x25519 pub at iter %d", i)
		}
		seen[key] = struct{}{}
	}
}

func TestGenerateKeypair_DeterministicWithFixedReader(t *testing.T) {
	a, err := generateKeypair(seededReader(0x7E))
	if err != nil {
		t.Fatal(err)
	}
	b, err := generateKeypair(seededReader(0x7E))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.x25519.Bytes(), b.x25519.Bytes()) {
		t.Fatalf("x25519 scalars differ for same seed")
	}
	if !bytes.Equal(a.mlkem.Bytes(), b.mlkem.Bytes()) {
		t.Fatalf("mlkem seeds differ for same seed")
	}
}

func TestGenerateKeypair_ReaderError(t *testing.T) {
	want := errors.New("nope")
	_, err := generateKeypair(failingReader{err: want})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped %v", err, want)
	}
}

// shortReader yields exactly n bytes then errors, to exercise the
// mlkem-seed branch after x25519 has consumed its bytes.
type shortReader struct {
	data []byte
	pos  int
	err  error
}

func (r *shortReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestGenerateKeypair_MLKEMSeedReadError(t *testing.T) {
	// x25519 needs 32 bytes; mlkem needs 64. Provide exactly 32 so the
	// second read fails.
	buf := bytes.Repeat([]byte{0xAA}, x25519KeySize)
	want := errors.New("truncated")
	_, err := generateKeypair(&shortReader{data: buf, err: want})
	if err == nil {
		t.Fatal("expected error on truncated mlkem seed")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped %v", err, want)
	}
}

func TestKeypair_PublicKey_MatchesInternal(t *testing.T) {
	k := mustGenerate(t)
	pub := k.PublicKey()
	if !bytes.Equal(pub.x25519.Bytes(), k.x25519.PublicKey().Bytes()) {
		t.Fatal("x25519 pub mismatch")
	}
	if !bytes.Equal(pub.mlkem.Bytes(), k.mlkem.EncapsulationKey().Bytes()) {
		t.Fatal("mlkem encap mismatch")
	}
}
