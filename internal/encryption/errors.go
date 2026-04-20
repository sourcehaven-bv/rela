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

// ErrRollbackDetected is returned when a sealed file's X-Rela-Version
// header carries a version LOWER than the highest this machine has
// already observed for this repo. Indicates either an adversary
// restored an older version of the file from cloud-side snapshot,
// or the local user intentionally reverted (in which case they
// should clear the per-repo XDG state).
var ErrRollbackDetected = errors.New("encryption: rollback detected: sealed file is older than last seen")

// ErrFileRelocated is returned when a sealed file's X-Rela-Path
// header does not match the path the file was read from.
// Indicates either an adversary renamed a sealed file on disk to
// impersonate another file, or a legitimate but uncoordinated
// manual rename (rare; recovery is re-seal under the new path).
var ErrFileRelocated = errors.New("encryption: sealed file path mismatch: possible swap or rename")

// IsRollbackDetected / IsFileRelocated mirror the other predicate
// helpers. Callers above cryptofs use these to surface human-
// readable errors to the CLI user without importing the sentinels.
func IsRollbackDetected(err error) bool { return errors.Is(err, ErrRollbackDetected) }
func IsFileRelocated(err error) bool    { return errors.Is(err, ErrFileRelocated) }
