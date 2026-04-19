package encryption

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpen_RoundTrip(t *testing.T) {
	dk, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	nonceSize, tagSize := aeadSizes()
	for _, size := range []int{0, 1, 15, 16, 17, 1024, 1 << 16} {
		plaintext := bytes.Repeat([]byte{byte(size)}, size)
		sealed, err := Seal(plaintext, dk)
		if err != nil {
			t.Fatalf("size=%d: Seal: %v", size, err)
		}
		if len(sealed) != nonceSize+size+tagSize {
			t.Fatalf("size=%d: sealed len = %d, want %d", size, len(sealed), nonceSize+size+tagSize)
		}
		got, err := Open(sealed, dk)
		if err != nil {
			t.Fatalf("size=%d: Open: %v", size, err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("size=%d: round-trip mismatch", size)
		}
	}
}

func TestSeal_NoncePrepended(t *testing.T) {
	dk := bytes.Repeat([]byte{0x01}, DataKeySize)
	pt := []byte("hello")
	sealed, err := sealWith(seededReader(0xAB), pt, dk)
	if err != nil {
		t.Fatal(err)
	}
	nonceSize, _ := aeadSizes()
	for i := range nonceSize {
		if sealed[i] != 0xAB {
			t.Fatalf("nonce[%d] = %#x, want 0xAB", i, sealed[i])
		}
	}
}

func TestSeal_WrongKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := Seal([]byte("x"), make([]byte, n))
		if err == nil {
			t.Fatalf("len=%d: expected error", n)
		}
	}
}

func TestSeal_EntropyError(t *testing.T) {
	dk := make([]byte, DataKeySize)
	want := errors.New("boom")
	_, err := sealWith(failingReader{err: want}, []byte("x"), dk)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped %v", err, want)
	}
}

func TestOpen_WrongKeyLength(t *testing.T) {
	nonceSize, tagSize := aeadSizes()
	sealed := make([]byte, nonceSize+tagSize)
	for _, n := range []int{0, 16, 31, 33, 64} {
		_, err := Open(sealed, make([]byte, n))
		if err == nil {
			t.Fatalf("len=%d: expected error", n)
		}
	}
}

func TestOpen_ShortCiphertext(t *testing.T) {
	dk := bytes.Repeat([]byte{0x02}, DataKeySize)
	nonceSize, tagSize := aeadSizes()
	minLen := nonceSize + tagSize
	for _, n := range []int{0, 1, minLen - 1} {
		_, err := Open(make([]byte, n), dk)
		if !errors.Is(err, ErrDecrypt) {
			t.Fatalf("len=%d: err = %v, want ErrDecrypt", n, err)
		}
	}
}

func TestOpen_Tamper(t *testing.T) {
	dk := bytes.Repeat([]byte{0x03}, DataKeySize)
	sealed, err := Seal([]byte("hello world, tamper test"), dk)
	if err != nil {
		t.Fatal(err)
	}
	// Flip one byte at each position up to the first 32.
	for i := range min(32, len(sealed)) {
		mutated := append([]byte(nil), sealed...)
		mutated[i] ^= 0x01
		if _, err := Open(mutated, dk); !errors.Is(err, ErrDecrypt) {
			t.Fatalf("flip byte %d: err = %v, want ErrDecrypt", i, err)
		}
	}
}

func TestOpen_WrongKey(t *testing.T) {
	a := bytes.Repeat([]byte{0x04}, DataKeySize)
	b := bytes.Repeat([]byte{0x05}, DataKeySize)
	sealed, err := Seal([]byte("payload"), a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open(sealed, b)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestSealOpen_DeterministicWithFixedReader(t *testing.T) {
	dk := bytes.Repeat([]byte{0x06}, DataKeySize)
	pt := []byte("deterministic")
	a, _ := sealWith(seededReader(0xCD), pt, dk)
	b, _ := sealWith(seededReader(0xCD), pt, dk)
	if !bytes.Equal(a, b) {
		t.Fatal("same inputs must give same sealed bytes")
	}
}
