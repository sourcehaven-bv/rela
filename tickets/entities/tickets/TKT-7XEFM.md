---
id: TKT-7XEFM
type: ticket
title: Port internal/encryption crypto primitives from prior branch
kind: enhancement
priority: high
effort: s
status: ready
---

Copy the cryptographic primitives from the prior `encryption-combined` branch into develop. Scope is strictly the leaf package — no consumers yet.

## What to port verbatim

From `internal/encryption/` on `encryption-combined`:

- `keypair.go` — X25519 + ML-KEM-768 Keypair / PublicKey, GenerateKeypair, `equal()` helper.
- `pem.go` — MarshalPublicKeyPEM, MarshalPrivateKeyPEM, ParsePublicKeyPEM, ParsePrivateKeyPEM.
- `aead.go` — AES-256-GCM Seal/Open with fresh-nonce discipline, shared length constants.
- `wrap.go` — WrapKey / UnwrapKey with the `RLAE` magic + wrap version byte + hybrid KEM construction.
- `keyring.go` — Keyring type, LoadKeyring(keysDir, privPath), Recipient/Identities/HasPrivateKey/LocalIdentity/Unwrap.
- `loader.go` — LoadFromDir with the $RELA_KEY_FILE → .rela/key → ~/.config/rela/key precedence.
- `errors.go` — typed sentinels (ErrNoPrivateKey, ErrBadPEM, ErrBadBlob, ErrDecrypt, ErrNoMatchingKey) and the `errDecryptGCM(_ error)` discipline.
- `doc.go` — package-level documentation.
- `must.go` / `mustStdlibContract` — if the helper existed; otherwise inline.

All associated `_test.go` files port with zero edits.

## What NOT to port

- `opaque.go` / `Opaque` type — no partial-decrypt scenario in whole-file sealing (a file either unseals or it doesn't).
- Anything metamodel-aware.
- Anything group-aware.

## Acceptance criteria

1. `internal/encryption/` compiles as a leaf package (zero imports from other `internal/` packages).
2. `go-arch-lint` passes (add component entry if arch config needs it).
3. All ported tests pass including the reflective `TestSecretTypes_NoStringMethods` and `TestErrDecryptGCM_DoesNotWrapCause`.
4. `just ci` passes end-to-end.
5. Package is completely unused by the rest of the tree (no consumers yet).
