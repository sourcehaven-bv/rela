package encryption

import (
	"bytes"
	"encoding/pem"
	"errors"
	"testing"
)

func TestMarshalPrivateKeyPEM_RoundTrip(t *testing.T) {
	k := mustGenerate(t)
	out, err := MarshalPrivateKeyPEM(k)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePrivateKeyPEM(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(parsed.x25519.Bytes(), k.x25519.Bytes()) {
		t.Fatal("x25519 scalar differs after round-trip")
	}
	if !bytes.Equal(parsed.mlkem.Bytes(), k.mlkem.Bytes()) {
		t.Fatal("mlkem seed differs after round-trip")
	}
	// Marshal→parse→marshal should be byte-identical.
	again, err := MarshalPrivateKeyPEM(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, again) {
		t.Fatal("re-marshal not byte-identical")
	}
}

func TestMarshalPublicKeyPEM_RoundTrip(t *testing.T) {
	k := mustGenerate(t)
	pub := k.PublicKey()
	out, err := MarshalPublicKeyPEM(pub)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePublicKeyPEM(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(parsed.x25519.Bytes(), pub.x25519.Bytes()) {
		t.Fatal("x25519 pub differs")
	}
	if !bytes.Equal(parsed.mlkem.Bytes(), pub.mlkem.Bytes()) {
		t.Fatal("mlkem encap differs")
	}
}

func TestMarshalPrivateKeyPEM_NilKeypair(t *testing.T) {
	if _, err := MarshalPrivateKeyPEM(nil); err == nil {
		t.Fatal("expected error for nil keypair")
	}
}

func TestMarshalPublicKeyPEM_NilPublic(t *testing.T) {
	if _, err := MarshalPublicKeyPEM(nil); err == nil {
		t.Fatal("expected error for nil public key")
	}
}

func TestParsePrivateKeyPEM_NoBlock(t *testing.T) {
	_, err := ParsePrivateKeyPEM([]byte("not a PEM file"))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePublicKeyPEM_NoBlock(t *testing.T) {
	_, err := ParsePublicKeyPEM([]byte("garbage"))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePrivateKeyPEM_WrongType(t *testing.T) {
	block := &pem.Block{Type: "SOMETHING ELSE", Bytes: bytes.Repeat([]byte{0}, privatePayloadSize)}
	_, err := ParsePrivateKeyPEM(pem.EncodeToMemory(block))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePublicKeyPEM_WrongType(t *testing.T) {
	block := &pem.Block{Type: "NOT A REAL TYPE", Bytes: bytes.Repeat([]byte{0}, publicPayloadSize)}
	_, err := ParsePublicKeyPEM(pem.EncodeToMemory(block))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePrivateKeyPEM_WrongLength(t *testing.T) {
	block := &pem.Block{Type: pemTypePrivateV1, Bytes: []byte("too short")}
	_, err := ParsePrivateKeyPEM(pem.EncodeToMemory(block))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePublicKeyPEM_WrongLength(t *testing.T) {
	block := &pem.Block{Type: pemTypePublicV1, Bytes: []byte("too short")}
	_, err := ParsePublicKeyPEM(pem.EncodeToMemory(block))
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}

func TestParsePrivateKeyPEM_BoundaryLengths(t *testing.T) {
	// Exact-boundary truncation: payload one byte short and one byte
	// over must both fail with ErrBadPEM. privatePayloadSize itself is
	// exercised by the round-trip test.
	for _, n := range []int{privatePayloadSize - 1, privatePayloadSize + 1} {
		block := &pem.Block{Type: pemTypePrivateV1, Bytes: make([]byte, n)}
		_, err := ParsePrivateKeyPEM(pem.EncodeToMemory(block))
		if !errors.Is(err, ErrBadPEM) {
			t.Fatalf("len=%d: err = %v, want ErrBadPEM", n, err)
		}
	}
}

func TestParsePublicKeyPEM_BoundaryLengths(t *testing.T) {
	for _, n := range []int{publicPayloadSize - 1, publicPayloadSize + 1} {
		block := &pem.Block{Type: pemTypePublicV1, Bytes: make([]byte, n)}
		_, err := ParsePublicKeyPEM(pem.EncodeToMemory(block))
		if !errors.Is(err, ErrBadPEM) {
			t.Fatalf("len=%d: err = %v, want ErrBadPEM", n, err)
		}
	}
}

func TestParsePublicKeyPEM_InvalidMLKEM(t *testing.T) {
	// Right length but an invalid ml-kem encap key. Start from a valid
	// public key and mutate the ml-kem portion in a way that fails the
	// structural check performed by NewEncapsulationKey768.
	k := mustGenerate(t)
	valid, err := MarshalPublicKeyPEM(k.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	// Baseline: the unmutated blob must parse. If this ever fails, the
	// mutation below is no longer measuring "valid → invalid" but
	// "invalid → invalid" and the negative assertion is meaningless.
	if _, perr := ParsePublicKeyPEM(valid); perr != nil {
		t.Fatalf("baseline parse failed: %v", perr)
	}
	block, _ := pem.Decode(valid)
	if block == nil {
		t.Fatal("decode failed")
	}
	// ML-KEM-768 encapsulation keys are rejected when the bytes don't
	// decode into valid polynomial coefficients. Flooding the encap
	// portion with 0xFF is reliably out of range for FIPS 203 encoding.
	for i := x25519KeySize; i < len(block.Bytes); i++ {
		block.Bytes[i] = 0xFF
	}
	mutated := pem.EncodeToMemory(block)
	_, err = ParsePublicKeyPEM(mutated)
	if !errors.Is(err, ErrBadPEM) {
		t.Fatalf("err = %v, want ErrBadPEM", err)
	}
}
