package encryption

import (
	"bytes"
	"errors"
	"testing"
)

func TestWrapKey_RoundTrip(t *testing.T) {
	k := mustGenerate(t)
	dk, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := WrapKey(dk, k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if len(wrapped) != wrappedBlobSize {
		t.Fatalf("wrapped len = %d, want %d", len(wrapped), wrappedBlobSize)
	}
	got, err := UnwrapKey(wrapped, k)
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	if !bytes.Equal(got, dk) {
		t.Fatal("unwrap != original")
	}
}

func TestWrapKey_MagicAndVersion(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, err := WrapKey(dk, k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if string(wrapped[:wrapMagicLen]) != wrapMagic {
		t.Fatalf("magic = %q, want %q", wrapped[:wrapMagicLen], wrapMagic)
	}
	if wrapped[wrapOffsetVersion] != wrapVersion {
		t.Fatalf("version = %#x, want %#x", wrapped[wrapOffsetVersion], wrapVersion)
	}
}

func TestWrapKey_NilRecipient(t *testing.T) {
	dk := make([]byte, DataKeySize)
	_, err := WrapKey(dk, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapKey_WrongDataKeyLength(t *testing.T) {
	k := mustGenerate(t)
	for _, n := range []int{0, 1, 16, 31, 33, 64} {
		_, err := WrapKey(make([]byte, n), k.PublicKey())
		if err == nil {
			t.Fatalf("len=%d: expected error", n)
		}
	}
}

func TestWrapKey_EntropyError(t *testing.T) {
	k := mustGenerate(t)
	dk := make([]byte, DataKeySize)
	_, err := wrapKey(failingReader{err: errors.New("boom")}, dk, k.PublicKey())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapKey_LowOrderRecipient(t *testing.T) {
	// A recipient whose X25519 public key is a low-order point causes
	// ECDH to fail inside WrapKey. In practice such a PublicKey would
	// need to be forged (ParsePublicKeyPEM validates length only for
	// X25519, but a well-behaved peer never generates one) — this
	// covers the adversarial-input branch.
	good := mustGenerate(t)
	dk, _ := NewDataKey()
	// Swap the recipient's x25519 for a low-order point.
	badX, err := ecdhX25519Zero()
	if err != nil {
		t.Fatal(err)
	}
	recipient := &PublicKey{x25519: badX, mlkem: good.PublicKey().mlkem}
	_, err = WrapKey(dk, recipient)
	if err == nil {
		t.Fatal("expected ECDH error on low-order recipient")
	}
}

func TestUnwrapKey_NilKeypair(t *testing.T) {
	b := make([]byte, wrappedBlobSize)
	copy(b, wrapMagic)
	b[wrapOffsetVersion] = wrapVersion
	_, err := UnwrapKey(b, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnwrapKey_BadLength(t *testing.T) {
	for _, n := range []int{0, 1, wrappedBlobSize - 1, wrappedBlobSize + 1} {
		_, err := UnwrapKey(make([]byte, n), mustGenerate(t))
		if !errors.Is(err, ErrBadBlob) {
			t.Fatalf("len=%d: err = %v, want ErrBadBlob", n, err)
		}
	}
}

func TestUnwrapKey_BadMagic(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	wrapped[0] ^= 0xFF
	_, err := UnwrapKey(wrapped, k)
	if !errors.Is(err, ErrBadBlob) {
		t.Fatalf("err = %v, want ErrBadBlob", err)
	}
}

func TestUnwrapKey_BadVersion(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	wrapped[wrapOffsetVersion] = 0xFE
	_, err := UnwrapKey(wrapped, k)
	if !errors.Is(err, ErrBadBlob) {
		t.Fatalf("err = %v, want ErrBadBlob", err)
	}
}

func TestUnwrapKey_CrossKey(t *testing.T) {
	a := mustGenerate(t)
	b := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, a.PublicKey())
	got, err := UnwrapKey(wrapped, b)
	if err == nil {
		t.Fatalf("cross-key must fail; got %x", got)
	}
	// A structurally-valid blob wrapped for A must fail AEAD
	// authentication when unwrapped with B's key → ErrDecrypt.
	// ErrBadBlob here would mean an inner layer rejected well-formed
	// bytes, which shouldn't happen.
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestUnwrapKey_TamperGCMBody(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	// Flip a byte in the wrapped-key portion (post-magic, post-ephpub,
	// post-mlkem).
	wrapped[wrapOffsetWrapped] ^= 0x01
	_, err := UnwrapKey(wrapped, k)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestUnwrapKey_TamperEphemeralPub(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	wrapped[wrapOffsetEphPub] ^= 0x01
	_, err := UnwrapKey(wrapped, k)
	// Mutating the ephemeral pubkey changes the ECDH shared secret,
	// derives a different KEK, and causes GCM auth failure on the
	// wrapped data key → ErrDecrypt. (A mutation that happens to
	// produce a low-order point would instead produce ErrBadBlob,
	// but that's vanishingly unlikely for a 1-bit flip.)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestUnwrapKey_LowOrderEphemeralPub(t *testing.T) {
	// Blob with an all-zero ephemeral pubkey triggers X25519 ECDH's
	// low-order-point rejection in UnwrapKey — the "malicious blob"
	// path that cannot be prevented by length/magic/version checks.
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	for i := wrapOffsetEphPub; i < wrapOffsetMLKEMCt; i++ {
		wrapped[i] = 0
	}
	_, err := UnwrapKey(wrapped, k)
	if !errors.Is(err, ErrBadBlob) {
		t.Fatalf("err = %v, want ErrBadBlob", err)
	}
}

func TestUnwrapKey_TamperMLKEMCt(t *testing.T) {
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	wrapped, _ := WrapKey(dk, k.PublicKey())
	wrapped[wrapOffsetMLKEMCt+10] ^= 0x01
	_, err := UnwrapKey(wrapped, k)
	// Flipping a byte in the ML-KEM ciphertext changes the shared
	// secret after decapsulation (ML-KEM decap doesn't return error
	// for ciphertext tampering — it returns a different key by
	// design, for implicit rejection). The derived KEK then fails
	// GCM auth on the wrapped blob → ErrDecrypt.
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestWrap_SameRecipient_DistinctBlobs(t *testing.T) {
	// Wrapping the same data key for the same recipient twice must
	// produce different blobs — the ephemeral X25519 key and ML-KEM
	// encapsulation must contribute fresh entropy. If these blobs
	// were ever equal, an attacker could equality-compare wrapped
	// keys across files.
	k := mustGenerate(t)
	dk, _ := NewDataKey()
	a, err := WrapKey(dk, k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	b, err := WrapKey(dk, k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two wraps of the same key for the same recipient produced identical blobs — entropy broken")
	}
}

func TestWrap_MultipleRecipients(t *testing.T) {
	// Same data key, wrapped for two independent keypairs, each
	// unwraps cleanly. This is the slice-2+ contract: multi-recipient
	// support composes from the V1 primitive without a shape change.
	a := mustGenerate(t)
	b := mustGenerate(t)
	dk, _ := NewDataKey()

	wA, err := WrapKey(dk, a.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	wB, err := WrapKey(dk, b.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: the two wrapped blobs differ (different ephemeral keys).
	if bytes.Equal(wA, wB) {
		t.Fatal("two wraps of the same key produced identical blobs — entropy broken")
	}

	gotA, err := UnwrapKey(wA, a)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := UnwrapKey(wB, b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotA, dk) || !bytes.Equal(gotB, dk) {
		t.Fatal("multi-recipient unwrap mismatch")
	}

	// Cross: A's wrapped blob must not open with B's key.
	if _, err := UnwrapKey(wA, b); err == nil {
		t.Fatal("cross-unwrap must fail")
	}
}

func TestWrapKey_FixedReader_RoundTrips(t *testing.T) {
	// We cannot assert byte-exact blob determinism because
	// mlkem.EncapsulationKey768.Encapsulate draws internal entropy
	// that callers can't override. But we can still verify that the
	// injected reader path works: generate a keypair + wrap a data
	// key with seeded entropy and confirm the result round-trips.
	k, err := generateKeypair(seededReader(0x11))
	if err != nil {
		t.Fatal(err)
	}
	dk := bytes.Repeat([]byte{0x22}, DataKeySize)
	wrapped, err := wrapKey(seededReader(0x33), dk, k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapKey(wrapped, k)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dk) {
		t.Fatal("round-trip mismatch")
	}
}
