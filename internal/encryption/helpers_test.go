package encryption

import (
	"crypto/ecdh"
	"io"
	"testing"
)

// fixedReader returns a reader yielding the given bytes in a loop.
// Deterministic for tests; never exported.
type fixedReader struct {
	src []byte
	pos int
}

func newFixedReader(src []byte) *fixedReader {
	if len(src) == 0 {
		panic("fixedReader: empty source")
	}
	return &fixedReader{src: src}
}

func (r *fixedReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.src[r.pos%len(r.src)]
		r.pos++
	}
	return len(p), nil
}

// seededReader fills with a deterministic repeating pattern from seed.
func seededReader(seed byte) io.Reader {
	return newFixedReader([]byte{seed})
}

// mustGenerate panics on error — tests only.
func mustGenerate(t *testing.T) *Keypair {
	t.Helper()
	k, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	return k
}

// ecdhX25519Zero returns an X25519 public key with the all-zero
// encoding — a low-order point that ECDH rejects. Used by tests that
// exercise adversarial-input branches.
func ecdhX25519Zero() (*ecdh.PublicKey, error) {
	return ecdh.X25519().NewPublicKey(make([]byte, x25519KeySize))
}

// aeadSizes queries the stdlib GCM AEAD for its nonce size and tag
// overhead. Tests use these rather than hardcoding — so we're not
// duplicating magic numbers.
func aeadSizes() (nonceSize, tagSize int) {
	aead := newGCM(make([]byte, DataKeySize))
	return aead.NonceSize(), aead.Overhead()
}
