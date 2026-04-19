package encryption

import "errors"

// Package error predicates.
//
// Consumers classify errors via these three predicates rather than by
// comparing against sentinel values. Predicate-only API prevents the
// prior C1 regression where a call site accidentally collapsed tamper
// (decrypt failure) into "no matching key" by conflating sentinels.

// ErrNoMatchingKey is returned when no loaded identity matches the
// recipient list of the sealed blob. The blob is well-formed; the
// local user simply is not authorized to read it.
var ErrNoMatchingKey = errors.New("encryption: no matching identity for any recipient")

// ErrCorrupted is returned when a sealed blob is malformed or has been
// tampered with. AEAD authentication failure, header parse failure,
// and truncated input all surface as this error.
var ErrCorrupted = errors.New("encryption: sealed blob is corrupted or malformed")

// ErrNoPrivateKey is returned when Unseal is called but no local
// identity is loaded at all. Distinct from ErrNoMatchingKey (which
// means an identity IS loaded, just not one in the recipient set).
var ErrNoPrivateKey = errors.New("encryption: no private identity loaded")

// IsNoMatchingKey reports whether err indicates the loaded identity
// is not among the blob's recipients.
func IsNoMatchingKey(err error) bool { return errors.Is(err, ErrNoMatchingKey) }

// IsCorrupted reports whether err indicates a tampered or malformed
// sealed blob.
func IsCorrupted(err error) bool { return errors.Is(err, ErrCorrupted) }

// IsNoPrivateKey reports whether err indicates no local identity is
// loaded at all.
func IsNoPrivateKey(err error) bool { return errors.Is(err, ErrNoPrivateKey) }
