// Package encryption is a thin facade over filippo.io/age for at-rest
// encryption of rela's entity, relation, and attachment files.
//
// The package intentionally does NOT roll its own crypto: age handles
// the envelope, per-file key, AEAD payload, and wire-format versioning.
// Our code concerns itself with (a) loading a keyring from the rela
// project layout, (b) invoking age.Encrypt / age.Decrypt with sensible
// defaults, and (c) classifying age's errors into three consumer-facing
// predicates (IsNoMatchingKey, IsCorrupted, IsNoPrivateKey).
//
// See DEC-D5P4X for why we chose age over a custom envelope.
//
// v1 uses age's built-in X25519 recipients. Post-quantum hybrid support
// will land as an age recipient plugin in a follow-up ticket.
package encryption
