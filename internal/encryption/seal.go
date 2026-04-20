package encryption

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
)

// SealedMagic is the prefix every age-sealed blob starts with. Used by
// fsstore to detect sealed vs cleartext files without attempting
// decryption, for partial-encryption invariant checks.
const SealedMagic = "age-encryption.org/v1\n"

// Seal encrypts plaintext for every recipient. The output is a
// self-contained age blob: header, recipient stanzas, payload. Any
// recipient's matching Identity can Unseal it.
func Seal(plaintext []byte, recipients []Recipient) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, errors.New("encryption: Seal requires at least one recipient")
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipientsAsAge(recipients)...)
	if err != nil {
		return nil, fmt.Errorf("encryption: seal: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("encryption: seal write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("encryption: seal close: %w", err)
	}
	return buf.Bytes(), nil
}

// Unseal decrypts a sealed blob using identity.
//
// Errors are classified into three categories, distinguishable via
// IsNoMatchingKey / IsCorrupted / IsNoPrivateKey. Tamper never
// collapses into no-matching-key: a blob addressed to identity but
// with a corrupted payload returns ErrCorrupted, not ErrNoMatchingKey.
func Unseal(blob []byte, identity Identity) ([]byte, error) {
	if identity == nil {
		return nil, ErrNoPrivateKey
	}
	if !LooksSealed(blob) {
		return nil, fmt.Errorf("%w: missing age header", ErrCorrupted)
	}
	r, err := age.Decrypt(bytes.NewReader(blob), identity.ageIdentity())
	if err != nil {
		return nil, classifyDecryptErr(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		// Payload-AEAD failures surface here (after Decrypt returned
		// a reader successfully). Classify as corruption, not no-match.
		return nil, fmt.Errorf("%w: %w", ErrCorrupted, err)
	}
	return out, nil
}

// LooksSealed reports whether blob starts with the age header. Used
// by fsstore to detect sealed files. Does NOT validate the payload.
func LooksSealed(blob []byte) bool {
	return bytes.HasPrefix(blob, []byte(SealedMagic))
}

// classifyDecryptErr maps age.Decrypt's errors onto our predicates.
//
// age returns ErrIncorrectIdentity when none of the passed identities
// can unwrap any recipient stanza. We map that to ErrNoMatchingKey.
//
// Everything else (header parse, stanza parse, AEAD failure) is
// classified as ErrCorrupted. This is deliberately broad: we prefer
// false-positive "corrupted" reports to the C1 failure mode where
// tamper silently becomes no-matching-key.
func classifyDecryptErr(err error) error {
	if errors.Is(err, age.ErrIncorrectIdentity) {
		return fmt.Errorf("%w: %w", ErrNoMatchingKey, err)
	}
	return fmt.Errorf("%w: %w", ErrCorrupted, err)
}
