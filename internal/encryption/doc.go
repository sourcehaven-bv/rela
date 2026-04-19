// Package encryption provides at-rest encryption primitives for rela.
//
// # Threat model
//
// Encryption protects data in shared git repositories, cloud storage,
// backups, and forks. It does NOT protect:
//
//   - data in memory,
//   - file names, entity IDs, or metamodel structure,
//   - relation topology,
//   - the local private key file (it is not passphrase-protected in V1).
//
// Forward secrecy against compromised private keys is not provided.
// Removing a member does not scrub prior content from clones they made.
//
// # Construction
//
// Hybrid X25519 + ML-KEM-768. Each wrap call generates an ephemeral
// X25519 keypair and performs ML-KEM-768 encapsulation against the
// recipient. The two shared secrets are concatenated and fed to
// HKDF-SHA256 with info string "rela-encryption v1" to derive a
// 32-byte key-encryption key (KEK). The KEK wraps a fresh 32-byte data
// key with AES-256-GCM. The data key protects payloads with AES-256-GCM
// (Seal/Open).
//
// The HKDF construction is inspired by RFC 9180 §4.1
// LabeledExtract/LabeledExpand. It is not HPKE proper — a simplified
// form chosen for V1. A future swap to full HPKE would bump the
// magic/version together.
//
// # Nonce discipline
//
// Seal prepends a fresh random 12-byte nonce per call. Callers must not
// reuse a data key beyond 2^32 Seal calls (AES-GCM birthday bound).
// Because NewDataKey is cheap, the intended pattern is one data key
// per encrypted artifact, which keeps the bound unreachable.
//
// # Error observability
//
// ErrBadBlob vs ErrDecrypt is not a cryptographically sensitive
// distinction — the blob format is public. Callers should not build UX
// that exposes this distinction to untrusted input.
package encryption
