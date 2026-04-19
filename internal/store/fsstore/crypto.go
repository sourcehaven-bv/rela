package fsstore

import (
	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// Crypto is the fsstore-side view of the encryption boundary. It is
// always non-nil on a live FSStore: when the repo is not encryption-
// enabled, an identityCrypto is installed so that Seal/Unseal are
// no-ops and the cleartext path doesn't have to branch on nil.
//
// Error classification is done via the three predicates exported by
// internal/encryption (IsNoMatchingKey, IsCorrupted, IsNoPrivateKey).
// No sentinel values are exported from this interface; fsstore
// callers should use the predicates to avoid the C1 class of bugs
// where tamper silently collapses into "no matching key."
type Crypto interface {
	// Seal produces the on-disk bytes for a marshaled file. The
	// output MUST be self-describing: a call to Unseal on the exact
	// bytes returned by Seal must round-trip.
	Seal(plaintext []byte) ([]byte, error)

	// Unseal inverts Seal. For identityCrypto this is an identity
	// function; for the real age implementation this delegates to
	// encryption.Unseal with the loaded local identity.
	Unseal(blob []byte) ([]byte, error)
}

// identityCrypto is installed when the repo is not encryption-enabled.
// All methods are side-effect-free and allocation-free.
type identityCrypto struct{}

// IdentityCrypto returns the no-op Crypto used when encryption is
// disabled. Exported so callers wiring up fsstore (workspace, tests)
// have an explicit "cleartext mode" value rather than relying on
// nil semantics.
func IdentityCrypto() Crypto { return identityCrypto{} }

func (identityCrypto) Seal(p []byte) ([]byte, error)   { return p, nil }
func (identityCrypto) Unseal(p []byte) ([]byte, error) { return p, nil }

// isCleartextMode reports whether c is the no-op identityCrypto.
// Used by verifyEncryptionConsistency to choose which invariant to
// check. Kept as a free function rather than an interface method
// because the mode-selection concern is specific to fsstore's open
// path and does not belong in the Crypto contract.
func isCleartextMode(c Crypto) bool {
	_, ok := c.(identityCrypto)
	return ok
}

// ageCrypto wraps an encryption.Keyring into a Crypto. Writes seal
// for every loaded recipient; reads unseal with the loaded local
// identity.
type ageCrypto struct {
	kr *encryption.Keyring
}

// NewAgeCrypto returns a Crypto that seals blobs for every recipient
// in kr and unseals using kr's local identity. If kr has no
// recipients, Seal returns an error (cannot seal for nobody). If kr
// has no local identity, Unseal returns ErrNoPrivateKey.
func NewAgeCrypto(kr *encryption.Keyring) Crypto {
	return &ageCrypto{kr: kr}
}

func (a *ageCrypto) Seal(p []byte) ([]byte, error) {
	return encryption.Seal(p, a.kr.Recipients())
}

func (a *ageCrypto) Unseal(blob []byte) ([]byte, error) {
	return encryption.Unseal(blob, a.kr.Identity())
}
