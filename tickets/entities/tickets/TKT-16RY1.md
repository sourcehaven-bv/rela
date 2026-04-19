---
id: TKT-16RY1
type: ticket
title: Add internal/encryption crypto primitives (slice 1)
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

First slice of the encryption feature: a self-contained `internal/encryption/`
package providing crypto primitives. No rela coupling in the core library; a
thin `LoadFromDir` helper encodes rela conventions without importing rela types
(mirrors `internal/ai/loader.go`).

## Scope

### In scope

- **Hybrid keypair generation**: X25519 + ML-KEM-768 combined. Versioned PEM-encoded keys.
- **Data key generation**: `NewDataKey()` — centralised entropy source for 32-byte AES-256 keys.
- **Key wrapping**: wrap/unwrap a data key for a single recipient using the hybrid envelope. Length-prefixed, magic-headered blob format.
- **AEAD**: AES-256-GCM `Seal` / `Open` with documented nonce discipline (random 12-byte IV prepended to ciphertext).
- **Keyring**: loads recipient public keys from a directory; loads an optional local private key; exposes an `Unwrap` method.
- **Rela-convention loader**: `LoadFromDir(relaDir)` applies the env-var → `.rela/key` → `~/.config/rela/key` precedence without importing any rela types.
- **Typed sentinel errors**: `ErrNoPrivateKey`, `ErrBadPEM`, `ErrBadBlob`, `ErrDecrypt` — usable with `errors.Is`.
- **Redaction discipline**: no key material in error messages; table-driven leak test (mirrors `ai/redact_test.go`).
- **Unit tests**: round-trip, tamper detection, malformed inputs, cross-key failure, precedence, redaction. Target 100% coverage.

### Out of scope (later slices)

- Metamodel parsing of `encrypted:` declarations
- Group resolution (lives with metamodel / config at the wiring site — NOT in this package)
- `fsstore` integration (read/write of `!enc` tags)
- CLI commands (`rela keys generate`, etc.)
- Key version tracking and rotation
- Data-entry / MCP / desktop wiring

## Design Sketch

```go
// Package encryption provides at-rest encryption primitives for rela.
//
// Threat model: protects data in shared git repositories. Does NOT protect
// data in memory, file names, entity IDs, or metamodel structure. Does NOT
// passphrase-protect the local private key (the threat model is remote
// storage, not local filesystem).
package encryption

// Keypair is a hybrid X25519 + ML-KEM-768 private key.
type Keypair struct { /* private fields */ }

func GenerateKeypair() (*Keypair, error)
func (k *Keypair) PublicKey() *PublicKey

// PublicKey is the recipient-facing half of a hybrid keypair.
type PublicKey struct { /* private fields */ }

// PEM marshaling: symmetric package-level functions (stdlib pattern).
func MarshalPrivateKeyPEM(k *Keypair) ([]byte, error)
func ParsePrivateKeyPEM(data []byte) (*Keypair, error)
func MarshalPublicKeyPEM(p *PublicKey) ([]byte, error)
func ParsePublicKeyPEM(data []byte) (*PublicKey, error)

// Data key handling — centralise entropy in one place.
const DataKeySize = 32
func NewDataKey() ([]byte, error)

// Wrap/unwrap a data key for a single recipient.
func WrapKey(dataKey []byte, recipient *PublicKey) ([]byte, error)
func UnwrapKey(wrapped []byte, k *Keypair) ([]byte, error)

// AEAD: match stdlib crypto/cipher naming. Seal prepends a random 12-byte
// nonce to the ciphertext. Callers MUST NOT reuse a data key beyond 2^32
// calls (AES-GCM birthday bound).
func Seal(plaintext, dataKey []byte) ([]byte, error)
func Open(ciphertext, dataKey []byte) ([]byte, error)

// Keyring holds loaded recipients and (optionally) a local private key.
type Keyring struct { /* private fields */ }

func LoadKeyring(keysDir, privateKeyPath string) (*Keyring, error)
func (kr *Keyring) Recipient(id string) (*PublicKey, bool)
func (kr *Keyring) Identities() []string
func (kr *Keyring) Unwrap(wrapped []byte) ([]byte, error)   // uses local private key; returns ErrNoPrivateKey if absent

// Rela-convention entry point (no rela imports). Takes projectRoot and
// derives <projectRoot>/.rela internally. Resolves the private key from
// $RELA_KEY_FILE, <projectRoot>/.rela/key, or ~/.config/rela/key.
// Recipients come from <projectRoot>/keys.
func LoadFromDir(projectRoot string) (*Keyring, error)

var (
    ErrNoPrivateKey = errors.New("encryption: no private key configured")
    ErrBadPEM       = errors.New("encryption: malformed PEM")
    ErrBadBlob      = errors.New("encryption: malformed wrapped blob")
    ErrDecrypt      = errors.New("encryption: decryption failed")
)
```

## Files

- `internal/encryption/doc.go` — package docstring with threat model
- `internal/encryption/keypair.go` — keygen, `Keypair`, `PublicKey`
- `internal/encryption/pem.go` — marshal/parse functions
- `internal/encryption/wrap.go` — hybrid wrap/unwrap + blob format
- `internal/encryption/aead.go` — Seal/Open
- `internal/encryption/datakey.go` — `NewDataKey`, `DataKeySize`, package-private `randReader` for testability
- `internal/encryption/keyring.go` — `Keyring` type, `LoadKeyring`
- `internal/encryption/loader.go` — `LoadFromDir` (rela conventions, no rela imports)
- `internal/encryption/errors.go` — sentinel errors
- `internal/encryption/redact.go` — helper to scrub key material from errors (mirrors `ai/redact.go`)
- `*_test.go` for each — target 100% coverage

## Approach

Use `crypto/mlkem` from the Go standard library (Go 1.24+) for ML-KEM-768. Use
`crypto/ecdh` for X25519. Combine both shared secrets with HKDF-SHA256 to derive
the key-encryption key. If stdlib `crypto/mlkem` is not in the project's
toolchain, fall back to `github.com/cloudflare/circl/kem/mlkem/mlkem768`.

The HKDF `info` parameter is a named constant `hkdfInfoV1 = "rela-encryption
v1"`; salt is nil (RFC 5869 default). The construction is inspired by RFC 9180
§4.1 `LabeledExtract`/`LabeledExpand` but is a simplified form, not HPKE proper
— godoc calls this out so a future swap to HPKE can bump the version cleanly. No
user-supplied salt.

### Blob format (length-prefixed, versioned)

```
| magic "RLAE" (4B) | version (1B) | X25519 ephemeral pubkey (32B) | ML-KEM-768 ct (1088B) | wrapped key + GCM tag (48B) |
```

Total: 1173 bytes. Fixed offsets after `magic+version`. `ParseWrapped` validates
magic, version, and total length — rejects with `ErrBadBlob`.

### PEM block types (versioned)

- `RELA X25519-MLKEM768 PRIVATE KEY V1`
- `RELA X25519-MLKEM768 PUBLIC KEY V1`
- `RELA WRAPPED KEY V1`

Explicit scheme + version in the type string — future algorithm changes produce
distinct PEM types so old keys can be rejected cleanly.

### AEAD nonce discipline

`Seal(plaintext, dataKey)` generates a fresh random 12-byte nonce via
`crypto/rand` and prepends it to the ciphertext. Output layout: `nonce (12B) ||
ciphertext || GCM tag (16B)`. Godoc states the per-key message ceiling (2^32).
No nonce counter is exposed to callers — nonce reuse is impossible when callers
follow the contract (new data key per file).

### Entropy source

Stdlib style (mirrors `crypto/ecdsa.SignASN1`): exported zero-arg functions
(`GenerateKeypair`, `NewDataKey`, `Seal`, `WrapKey`) wrap unexported helpers
that take an `io.Reader` (`generateKeypair(r io.Reader)`, etc.). Tests call the
unexported variants directly — no package-global state, no test-only setter, no
`t.Cleanup` choreography.

### Redaction

Two-part discipline (deliberately *not* modelled on `ai/redact.go` — the threat
shape is different):

1. **Type-level hiding**: secret types (`Keypair`, `PublicKey`, `DataKey`, and
any `[]byte` wrapper around a scalar, data key, plaintext, or ciphertext) have
no `String()`, `GoString()`, or `MarshalJSON` methods. `fmt.Errorf("%v", k)`
prints the default non-revealing form. Where raw `[]byte` is in scope, a `safe(b
[]byte) string` helper renders `"<N bytes>"`.
2. **Centralised error constructors**: ~6 constructors (`errBadBlobAt(offset)`,
`errDecryptGCM(cause)`, etc.) are the only sites that build sentinel-wrapped
errors. The leak test asserts on these constructors directly, with distinctive
byte patterns, rather than trying to enumerate every call site.

Note: `ErrBadBlob` vs `ErrDecrypt` is not a security-sensitive distinction — the
blob format is public. Godoc documents this so callers don't expose the
distinction in UX fed by untrusted input.

### Zeroing

Best-effort zeroing of private scalars after unwrap, documented as best-effort.
Helper `zero(b []byte)`. No safety claim — GC may move memory.

## Acceptance Criteria

1. `GenerateKeypair()` → keypair survives PEM round-trip (marshal → parse → marshal is identical).
2. `WrapKey(dk, pub)` → `UnwrapKey(wrapped, priv)` recovers the original `dk` for the matching keypair.
3. `UnwrapKey` with a non-matching private key returns `ErrDecrypt` (never returns garbage data).
4. `Seal` / `Open` round-trip arbitrary byte slices, with the prepended nonce contract documented and enforced.
5. Tampering with any byte of a wrapped blob or GCM output is detected (returns `ErrBadBlob` or `ErrDecrypt`).
6. Blob parser rejects wrong magic, wrong version, wrong length with `ErrBadBlob`.
7. PEM parser rejects unknown block types and wrong versions with `ErrBadPEM`.
8. `LoadKeyring` loads recipients from a keys directory; filename (without `.pub`) is the identity. Non-PEM files return `ErrBadPEM` with the filename.
9. `LoadFromDir(projectRoot)` derives `<projectRoot>/.rela` internally and resolves private key precedence: `$RELA_KEY_FILE` → `<projectRoot>/.rela/key` → `~/.config/rela/key`. Missing private key is not an error — `Keyring.Unwrap` later returns `ErrNoPrivateKey`.
10. `Keyring.Recipient(id)`, `Identities()`, and `Unwrap(wrapped)` work correctly; no internal map is exposed.
11. No error path embeds key material, plaintext, or ciphertext. Leak test asserts this on every centralised error constructor with distinctive byte patterns; secret types have no `String`/`GoString`/`MarshalJSON` methods.
12. `NewDataKey()` produces 32 bytes via the unexported reader-taking helper; test verifies deterministic output by calling the helper with a fixed reader.
13. Package coverage is 100% (measured by `go test -cover`). No `coverage-ignore` comments in this package.

## Test Plan

- **Keypair**: keygen uniqueness (10 distinct keypairs), PEM round-trip, malformed PEM, wrong PEM block type, wrong PEM version.
- **PEM marshal/parse**: symmetric round-trip, rejection of mutations.
- **Wrap/unwrap**: round-trip, cross-key failure (returns `ErrDecrypt`, not garbage), malformed wrapped blob (truncation, magic, version).
- **AEAD**: round-trip, nonce-prepend contract, tamper detection (flip each of the first N byte positions), short ciphertext, wrong key.
- **DataKey**: length, randomness (via unexported helper with fixed reader), distinct across calls.
- **Keyring**: load from temp dir fixture; empty directory; single recipient; many recipients; non-PEM file; unknown recipient lookup.
- **Multi-recipient end-to-end**: generate two keypairs; wrap the same data key for both; each unwraps to the original. Proves the primitive composes for slice 2+ multi-recipient use without a later shape change.
- **LoadFromDir**: each of the three precedence paths (env set, project-local, user default); all missing (no error, `Unwrap` returns `ErrNoPrivateKey`); explicit env path pointing at nonexistent file → error.
- **Unwrap via Keyring**: delegates to `UnwrapKey`; returns `ErrNoPrivateKey` when no private key loaded.
- **Error redaction (leak test)**: table-driven over the centralised error constructors with distinctive byte patterns; assert none of those bytes appear in any `err.Error()` output.
- **Sentinel errors**: `errors.Is` for each documented sentinel.

All tests deterministic (entropy swapped where needed) — no flaky behaviour
allowed.

## Dependencies

None — this is the first slice, lays the foundation for all subsequent slices.

## Risk Assessment

- **Low**: pure library, no integration points. Algorithms are NIST-standardised (ML-KEM) or long-settled (X25519, AES-GCM, HKDF).
- **Crypto-specific risk**: nonce reuse in AES-GCM is catastrophic. Mitigated by (a) random nonces per `Seal` call, (b) documented per-key message ceiling, (c) contract that callers use a fresh data key per file.
- **Go version risk**: `crypto/mlkem` is recent stdlib. Fallback to `circl` documented.

## Effort

m — 1 to 2 days. Most of the time is tests and redaction discipline; the crypto
itself is stdlib/circl calls.
