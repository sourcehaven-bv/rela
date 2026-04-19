package fsstore

import (
	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// Crypto is the consumer-defined interface that fsstore uses to turn
// the metamodel's encryption policy + a keyring into on-disk
// encrypt/decrypt operations. A nil *Config.Crypto preserves fsstore's
// cleartext-only behavior exactly — no encryption code runs on reads
// or writes.
//
// Adapters implementing this interface live outside fsstore (in
// internal/app/factory.go for production) so fsstore stays layered
// strictly below the metamodel package per the arch-lint rules.
type Crypto interface {
	// PropertyGroup reports whether the given property on the given
	// entity type should be encrypted, and for which group. An empty
	// group string with encrypted=false means "cleartext" — that
	// return shape matches metamodel.EntityDef.BodyGroup() for
	// consistency.
	PropertyGroup(entityType, property string) (group string, encrypted bool)

	// BodyGroup reports whether the entity's markdown body should be
	// encrypted, and for which group.
	BodyGroup(entityType string) (group string, encrypted bool)

	// Recipients returns the ordered identity list for the group. The
	// returned slice must not be mutated by fsstore. (_, false) when
	// the group is unknown.
	Recipients(group string) ([]string, bool)

	// Recipient returns the public key for an identity; used at wrap
	// time. (_, false) when the identity is unknown.
	Recipient(identity string) (*encryption.PublicKey, bool)

	// UnwrapAny attempts to unwrap one of the supplied per-identity
	// wrapped blobs using the local keyring. Returns the recovered
	// data key and the identity that matched.
	//
	// Error contract (see acceptance criteria 3 and 7):
	//   - encryption.ErrNoMatchingKey: none of the identities
	//     corresponded to a usable local private key. Partial-decrypt
	//     state — caller surfaces affected values as Opaque.
	//   - encryption.ErrDecrypt / encryption.ErrBadBlob: a candidate
	//     was attempted but the blob was corrupt — caller raises a
	//     CorruptedFile error.
	//   - encryption.ErrNoPrivateKey: no local private key loaded at
	//     all — caller raises a MissingKeyring error.
	UnwrapAny(wraps map[string][]byte) (dataKey []byte, matched string, err error)

	// HasPrivateKey reports whether the local keyring is capable of
	// decrypting anything. Used to distinguish MissingKeyring from
	// WrongKey at read time before any unwrap is attempted.
	HasPrivateKey() bool
}
