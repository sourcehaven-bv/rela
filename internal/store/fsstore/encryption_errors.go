package fsstore

import "fmt"

// EncryptionErrorKind classifies a problem encountered by the
// fsstore seal/unseal path. Callers use errors.As to extract
// structured context (Kind, Property, Cause) for UI display or
// programmatic branching.
type EncryptionErrorKind string

const (
	// ErrKindMissingKeyring: the file contains encrypted fields but
	// the store has no Crypto configured or no local private key.
	ErrKindMissingKeyring EncryptionErrorKind = "missing_keyring"

	// ErrKindCorruptedFile: an encrypted field or wrapped data key
	// failed structural or cryptographic integrity checks. Wraps the
	// underlying encryption.ErrDecrypt / ErrBadBlob as Cause.
	ErrKindCorruptedFile EncryptionErrorKind = "corrupted_file"

	// ErrKindOpaqueWrite: caller attempted to write an entity whose
	// opaque value has been mutated (replaced or moved to a property
	// whose group no longer matches the envelope), which we can't
	// re-seal without a data key.
	ErrKindOpaqueWrite EncryptionErrorKind = "opaque_write"

	// ErrKindUnknownGroup: entity frontmatter references a group name
	// that the Crypto layer doesn't know about.
	ErrKindUnknownGroup EncryptionErrorKind = "unknown_group"

	// ErrKindUnknownRecipient: encryption policy names a recipient
	// identity whose public key the Crypto layer can't resolve.
	ErrKindUnknownRecipient EncryptionErrorKind = "unknown_recipient"
)

// EncryptionError is the typed error surfaced by fsstore on
// encryption-related read/write failures. Cause is the underlying
// error (may be an encryption.Err*, nil for non-wrap failures).
type EncryptionError struct {
	Kind     EncryptionErrorKind
	Property string // empty for file-level errors (body, envelope)
	Cause    error
}

func (e *EncryptionError) Error() string {
	base := fmt.Sprintf("fsstore: encryption %s", e.Kind)
	if e.Property != "" {
		base += " (property " + e.Property + ")"
	}
	if e.Cause != nil {
		base += ": " + e.Cause.Error()
	}
	return base
}

func (e *EncryptionError) Unwrap() error { return e.Cause }

// Is supports errors.Is matching against the sentinel variables
// below: two EncryptionErrors match if they share the same Kind.
// This mirrors the GroupError pattern in internal/metamodel and
// lets callers write errors.Is(err, ErrMissingKeyring) instead of
// errors.As + comparing Kind manually.
func (e *EncryptionError) Is(target error) bool {
	t, ok := target.(*EncryptionError)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

// Sentinel matchers. Callers use errors.Is(err, ErrMissingKeyring)
// etc. to branch on failure mode without inspecting the full struct.
var (
	ErrMissingKeyring   = &EncryptionError{Kind: ErrKindMissingKeyring}
	ErrCorruptedFile    = &EncryptionError{Kind: ErrKindCorruptedFile}
	ErrOpaqueWrite      = &EncryptionError{Kind: ErrKindOpaqueWrite}
	ErrUnknownGroup     = &EncryptionError{Kind: ErrKindUnknownGroup}
	ErrUnknownRecipient = &EncryptionError{Kind: ErrKindUnknownRecipient}
)
