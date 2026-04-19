package encryption

import (
	"encoding/pem"
	"reflect"
	"strings"
	"testing"
)

func TestSafe_RendersLengthOnly(t *testing.T) {
	secret := []byte("this-is-secret-data-please-do-not-leak")
	got := safe(secret)
	if strings.Contains(got, "secret") {
		t.Fatalf("safe() leaked content: %q", got)
	}
	if !strings.Contains(got, "bytes") {
		t.Fatalf("safe() missing length marker: %q", got)
	}
}

// TestRedaction_NoLeaks walks error paths that handle sensitive byte
// slices and asserts the sensitive bytes never appear in err.Error().
func TestRedaction_NoLeaks(t *testing.T) {
	// Distinctive byte patterns that would be obviously visible if
	// they leaked into an error string.
	const marker1 = "SENSITIVE-PLAINTEXT-XYZ"
	const marker2 = "SECRET-DATA-KEY-ABC"
	secretPlain := []byte(marker1)
	secretKey := []byte(marker2)

	assertNoLeak := func(t *testing.T, name string, err error, markers ...string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		msg := err.Error()
		for _, m := range markers {
			if strings.Contains(msg, m) {
				t.Fatalf("%s: error leaked %q: %q", name, m, msg)
			}
		}
	}

	t.Run("WrapKey wrong data-key length", func(t *testing.T) {
		k := mustGenerate(t)
		_, err := WrapKey(secretKey, k.PublicKey())
		assertNoLeak(t, "WrapKey", err, marker2)
	})

	t.Run("Seal wrong data-key length", func(t *testing.T) {
		_, err := Seal(secretPlain, secretKey)
		assertNoLeak(t, "Seal", err, marker1, marker2)
	})

	t.Run("Open wrong data-key length", func(t *testing.T) {
		nonceSize, tagSize := aeadSizes()
		_, err := Open(make([]byte, nonceSize+tagSize), secretKey)
		assertNoLeak(t, "Open", err, marker2)
	})

	t.Run("UnwrapKey bad blob", func(t *testing.T) {
		k := mustGenerate(t)
		b := append([]byte("XXXX"), secretKey...)
		b = append(b, make([]byte, wrappedBlobSize-len(b))...)
		_, err := UnwrapKey(b, k)
		assertNoLeak(t, "UnwrapKey", err, marker2)
	})

	t.Run("ParsePrivateKeyPEM bad", func(t *testing.T) {
		junk := append([]byte("not-a-pem-"), secretPlain...)
		_, err := ParsePrivateKeyPEM(junk)
		assertNoLeak(t, "ParsePrivateKeyPEM", err, marker1)
	})

	t.Run("ParsePublicKeyPEM bad", func(t *testing.T) {
		junk := append([]byte("not-a-pem-"), secretPlain...)
		_, err := ParsePublicKeyPEM(junk)
		assertNoLeak(t, "ParsePublicKeyPEM", err, marker1)
	})

	t.Run("ParsePrivateKeyPEM wrong length", func(t *testing.T) {
		block := &pem.Block{Type: pemTypePrivateV1, Bytes: secretKey}
		_, err := ParsePrivateKeyPEM(pem.EncodeToMemory(block))
		assertNoLeak(t, "ParsePrivateKeyPEM length", err, marker2)
	})
}

// TestSecretTypes_NoStringMethods asserts secret-holding types define
// no Stringer-style methods that could leak their contents via default
// fmt verbs. This is the type-level half of the redaction discipline.
func TestSecretTypes_NoStringMethods(t *testing.T) {
	forbidden := []string{"String", "GoString", "MarshalJSON", "MarshalText", "Format"}
	types := []reflect.Type{
		reflect.TypeOf(Keypair{}),
		reflect.TypeOf(&Keypair{}),
		reflect.TypeOf(PublicKey{}),
		reflect.TypeOf(&PublicKey{}),
		reflect.TypeOf(Keyring{}),
		reflect.TypeOf(&Keyring{}),
	}
	for _, tp := range types {
		for _, name := range forbidden {
			if _, ok := tp.MethodByName(name); ok {
				t.Errorf("%s defines %s — would risk leaking secrets via fmt", tp, name)
			}
		}
	}
}
