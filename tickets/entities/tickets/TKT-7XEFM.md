---
id: TKT-7XEFM
type: ticket
title: Rewrite internal/encryption as an age-based leaf package
kind: enhancement
priority: high
effort: s
status: ready
---

Replace the ported custom-crypto primitives with a thin `filippo.io/age` facade. See DEC-D5P4X for the rationale.

## What the package exposes

- `Recipient` — wraps `age.Recipient` (v1: `*age.X25519Recipient`; future: a PQ plugin recipient).
- `Identity` — wraps `age.Identity` (v1: `*age.X25519Identity`).
- `Keyring` — loads public keys from `<repo>/keys/*.pub` and a local identity from a path. Methods: `Recipients()`, `HasIdentity()`, `LocalIdentity() string` (the matched public key's filename stem, for UX messages only).
- `Seal(plaintext []byte, recipients []Recipient) ([]byte, error)` — thin wrapper over `age.Encrypt` producing a self-contained blob.
- `Unseal(blob []byte, identity Identity) ([]byte, error)` — thin wrapper over `age.Decrypt`. Returns `ErrNoMatchingKey`, `ErrCorrupted`, or `ErrNoPrivateKey` (never collapses corruption into no-matching-key).
- `LoadFromDir(projectRoot string) (*Keyring, error)` — keeps the existing precedence chain for the local identity.
- `ParsePublicKey(pem []byte) (Recipient, error)` / `MarshalPublicKey(Recipient) []byte` — for `<repo>/keys/*.pub`.
- `GenerateIdentity() (Identity, error)` — for `rela keys generate`.

## What the package does NOT do

- No per-property encryption, no group awareness, no Opaque type.
- No custom wire format, magic bytes, version bytes, or envelope YAML schema.
- No recipient-set fingerprint.
- No ML-KEM-768 hybrid (follow-up; tracked separately as a post-quantum recipient plugin).

## What gets deleted from the port

From the current `internal/encryption/` (ported in the prior commit on this branch):

- `aead.go`, `aead_test.go` — age does AEAD.
- `assert.go`, `assert_test.go` — only used by the old hybrid wrap.
- `datakey.go`, `datakey_test.go` — age manages the file key.
- `wrap.go`, `wrap_test.go` — the entire X25519+ML-KEM hybrid construction.
- `pem.go`, `pem_test.go` — age public keys are ASCII; no PEM.
- Existing `keypair.go`, `keypair_test.go` — replaced by age identity type.
- Existing `keyring.go`, `keyring_test.go` — rewritten against age.
- Existing `errors.go`, `errors_test.go` — slimmed down to the three error predicates consumers care about.
- `redact.go`, `redact_test.go` — kept if still useful for Identity.String()/MarshalJSON; reviewed during rewrite.

## Error predicates (consumer-facing)

Three predicates, not sentinels:

```go
func IsNoMatchingKey(err error) bool    // local identity not in recipient set
func IsCorrupted(err error) bool        // ciphertext tampered / not a valid age blob
func IsNoPrivateKey(err error) bool     // no local identity loaded at all
```

These map onto age's error classification internally. Sentinels are NOT exported because predicate-only API prevents the "collapse into wrong error" bugs from the prior design (C1).

## Acceptance criteria

1. `internal/encryption/` depends only on stdlib + `filippo.io/age`. Arch-lint stays happy (pure leaf).
2. `Seal` produces output that `age -d` can decrypt with the matching identity; `Unseal` accepts output produced by the `age` CLI.
3. Tampered ciphertext (flip a byte in the payload region) produces an error for which `IsCorrupted(err)` is true; `IsNoMatchingKey(err)` is false.
4. Unseal with an identity not in the recipient set produces `IsNoMatchingKey` true; `IsCorrupted` false.
5. Unseal with no loaded identity produces `IsNoPrivateKey` true.
6. `Identity.String()` and `Identity.MarshalJSON` never reveal the secret scalar; reflective `TestSecretTypes_NoStringMethods` adapted to the new types.
7. All public API doc-comments mention that this is age-backed and reference DEC-D5P4X.
8. `just ci` passes end-to-end.
