package encryption

// Opaque represents a value that the local keyring could not decrypt.
// It preserves the wire-format ciphertext bytes so unmodified values
// can be re-emitted verbatim — a user without a group's private key
// can still edit cleartext fields and save without corrupting the
// encrypted ones.
//
// The zero value is not meaningful; construct via NewOpaque.
//
// Round-trip discipline:
//
//   - Opaque.String() returns "<encrypted>" so fmt verbs don't leak
//     ciphertext into logs or UIs.
//   - MarshalJSON produces the same redacted string.
//   - Bytes() returns an independent copy of the ciphertext for
//     general use.
//   - BorrowBytes() returns the underlying slice without copying for
//     performance-sensitive re-emit paths. The caller MUST NOT mutate
//     the returned slice.
type Opaque struct {
	ciphertext []byte
}

// NewOpaque wraps raw wire-format ciphertext bytes. The input is
// defensively copied so the Opaque value is immutable from the
// caller's perspective.
func NewOpaque(ciphertext []byte) Opaque {
	cp := make([]byte, len(ciphertext))
	copy(cp, ciphertext)
	return Opaque{ciphertext: cp}
}

// String returns a redacted placeholder. Implementations of
// fmt.Stringer on secret types are a deliberate anti-leak measure:
// any %v/%s verb on an Opaque prints "<encrypted>" rather than a
// struct dump with the raw ciphertext.
func (Opaque) String() string { return "<encrypted>" }

// MarshalJSON produces the same redacted string so cache files,
// API responses, and logs never serialise ciphertext bytes.
func (Opaque) MarshalJSON() ([]byte, error) {
	return []byte(`"<encrypted>"`), nil
}

// Bytes returns an independent copy of the ciphertext. Use this when
// the caller may mutate the returned slice or needs ownership
// semantics.
func (o Opaque) Bytes() []byte {
	cp := make([]byte, len(o.ciphertext))
	copy(cp, o.ciphertext)
	return cp
}

// BorrowBytes returns the underlying ciphertext slice WITHOUT copying.
// The caller MUST NOT mutate the returned slice. Use only on
// performance-sensitive paths (e.g. re-emit-on-write in fsstore) where
// the slice is immediately consumed and discarded. For any other use,
// call Bytes().
func (o Opaque) BorrowBytes() []byte {
	return o.ciphertext
}

// Len returns the length of the underlying ciphertext in bytes.
// Useful for tests and diagnostics without exposing the bytes.
func (o Opaque) Len() int { return len(o.ciphertext) }
