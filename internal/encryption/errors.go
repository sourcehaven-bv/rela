package encryption

import (
	"errors"
	"fmt"
)

var (
	ErrNoPrivateKey = errors.New("encryption: no private key configured")
	ErrBadPEM       = errors.New("encryption: malformed PEM")
	ErrBadBlob      = errors.New("encryption: malformed wrapped blob")
	ErrDecrypt      = errors.New("encryption: decryption failed")

	// ErrNoMatchingKey indicates that none of the wrapped-for identities
	// corresponded to the local private key. Distinct from ErrDecrypt /
	// ErrBadBlob (which signal genuine corruption): if the local key
	// legitimately isn't in the recipient set, the caller is simply not
	// authorized to read this blob.
	ErrNoMatchingKey = errors.New("encryption: no matching private key for any recipient")
)

// Wrapping convention: every constructor wraps its sentinel with %w so
// callers can use errors.Is. Causes from stdlib or lower layers are
// stringified with %s rather than wrapped with %w — the sentinel is
// the public contract; the cause is diagnostic context that we don't
// promise to keep stable and don't want to expose via errors.As.

func errBadPEM(filename string, cause error) error {
	if filename == "" {
		return fmt.Errorf("%w: %s", ErrBadPEM, cause.Error())
	}
	return fmt.Errorf("%w: %s: %s", ErrBadPEM, filename, cause.Error())
}

func errBadPEMType(gotType, wantType string) error {
	return fmt.Errorf("%w: block type %q, want %q", ErrBadPEM, gotType, wantType)
}

func errBadPEMLength(gotLen, wantLen int) error {
	return fmt.Errorf("%w: payload length %d, want %d", ErrBadPEM, gotLen, wantLen)
}

func errBadBlobMagic() error {
	return fmt.Errorf("%w: bad magic", ErrBadBlob)
}

func errBadBlobVersion(got byte) error {
	return fmt.Errorf("%w: version %d unsupported", ErrBadBlob, got)
}

func errBadBlobLength(got int) error {
	return fmt.Errorf("%w: length %d, want %d", ErrBadBlob, got, wrappedBlobSize)
}

// errDecryptGCM intentionally produces the bare ErrDecrypt sentinel
// and DROPS its argument. The name encodes the security invariant:
// an AEAD decryption failure must never be wrapped to carry the
// underlying cause. Exposing the cause could create a padding-oracle-
// adjacent side channel even though AEAD itself is safer than CBC —
// a helpful "wrong-key" distinction tells an attacker whether their
// guess was structurally plausible. See TestErrDecryptGCM_DoesNotWrapCause.
//
// The unused parameter is kept so call sites read naturally at
// wrap-call points: `return nil, errDecryptGCM(err)`.
func errDecryptGCM(_ error) error {
	return ErrDecrypt
}
