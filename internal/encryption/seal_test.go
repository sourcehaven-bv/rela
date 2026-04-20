package encryption

import (
	"bytes"
	"strings"
	"testing"
)

func TestSealUnseal_RoundTrip(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	plain := []byte("hello world")

	sealed, err := Seal(plain, []Recipient{alice.PublicRecipient(), bob.PublicRecipient()})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !LooksSealed(sealed) {
		t.Fatalf("Seal output missing age header")
	}

	for name, id := range map[string]Identity{"alice": alice, "bob": bob} {
		got, err := Unseal(sealed, id)
		if err != nil {
			t.Errorf("Unseal(%s): %v", name, err)
			continue
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("Unseal(%s) = %q, want %q", name, got, plain)
		}
	}
}

func TestSeal_NoRecipients(t *testing.T) {
	_, err := Seal([]byte("x"), nil)
	if err == nil {
		t.Fatal("Seal with no recipients should error")
	}
}

func TestUnseal_NotARecipient(t *testing.T) {
	alice := newTestIdentity(t)
	bob := newTestIdentity(t)
	eve := newTestIdentity(t)

	sealed, err := Seal([]byte("secret"), []Recipient{alice.PublicRecipient(), bob.PublicRecipient()})
	if err != nil {
		t.Fatal(err)
	}

	_, err = Unseal(sealed, eve)
	if !IsNoMatchingKey(err) {
		t.Errorf("IsNoMatchingKey(err) = false (err = %v)", err)
	}
	if IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = true (should be false for wrong-identity case)")
	}
}

func TestUnseal_NoIdentity(t *testing.T) {
	alice := newTestIdentity(t)
	sealed, err := Seal([]byte("secret"), []Recipient{alice.PublicRecipient()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Unseal(sealed, nil)
	if !IsNoPrivateKey(err) {
		t.Errorf("IsNoPrivateKey(err) = false (err = %v)", err)
	}
}

func TestUnseal_TamperedHeader(t *testing.T) {
	alice := newTestIdentity(t)
	sealed, err := Seal([]byte("secret"), []Recipient{alice.PublicRecipient()})
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte in the header.
	sealed[5] ^= 0x01

	_, err = Unseal(sealed, alice)
	if !IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = false (err = %v)", err)
	}
	if IsNoMatchingKey(err) {
		t.Errorf("tamper must not collapse into IsNoMatchingKey (err = %v)", err)
	}
}

func TestUnseal_TamperedPayload(t *testing.T) {
	// Regression guard: the local identity IS a recipient, but the
	// payload is flipped. The error MUST surface as corruption, not
	// "no matching key" — otherwise callers can't tell tampering
	// apart from authorization problems.
	alice := newTestIdentity(t)
	sealed, err := Seal([]byte("secret-payload-long-enough-to-have-payload-bytes"), []Recipient{alice.PublicRecipient()})
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte near the end to hit the payload, not the header.
	sealed[len(sealed)-5] ^= 0x01

	_, err = Unseal(sealed, alice)
	if !IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = false (err = %v)", err)
	}
	if IsNoMatchingKey(err) {
		t.Errorf("tampered-payload must not collapse into IsNoMatchingKey (err = %v)", err)
	}
}

func TestUnseal_NotAnAgeBlob(t *testing.T) {
	alice := newTestIdentity(t)
	_, err := Unseal([]byte("this is not age at all"), alice)
	if !IsCorrupted(err) {
		t.Errorf("IsCorrupted(err) = false (err = %v)", err)
	}
}

func TestLooksSealed(t *testing.T) {
	cases := map[string]bool{
		"":                             false,
		"hello":                        false,
		"age-encryption":               false,
		SealedMagic:                    true,
		SealedMagic + "rest":           true,
		" " + SealedMagic:              false,
		strings.TrimSpace(SealedMagic): false, // no trailing \n
	}
	for in, want := range cases {
		if got := LooksSealed([]byte(in)); got != want {
			t.Errorf("LooksSealed(%q) = %v, want %v", in, got, want)
		}
	}
}
