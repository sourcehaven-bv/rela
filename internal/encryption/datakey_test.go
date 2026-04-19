package encryption

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestNewDataKey_Length(t *testing.T) {
	k, err := NewDataKey()
	if err != nil {
		t.Fatalf("NewDataKey: %v", err)
	}
	if len(k) != DataKeySize {
		t.Fatalf("len=%d want %d", len(k), DataKeySize)
	}
}

func TestNewDataKey_Distinct(t *testing.T) {
	a, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatalf("two successive data keys are equal — entropy broken")
	}
}

func TestNewDataKey_DeterministicWithFixedReader(t *testing.T) {
	a, err := newDataKey(seededReader(0x42))
	if err != nil {
		t.Fatal(err)
	}
	b, err := newDataKey(seededReader(0x42))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("same seed must give same key")
	}
	wantFirst := byte(0x42)
	if a[0] != wantFirst {
		t.Fatalf("first byte = %#x, want %#x", a[0], wantFirst)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read(_ []byte) (int, error) { return 0, f.err }

func TestNewDataKey_ReaderError(t *testing.T) {
	want := errors.New("boom")
	_, err := newDataKey(failingReader{err: want})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want wrapped %v", err, want)
	}
}

// Compile-time assertion that io.Reader is the contract.
var _ io.Reader = failingReader{}
