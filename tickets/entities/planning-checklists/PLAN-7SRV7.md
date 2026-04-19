---
id: PLAN-7SRV7
type: planning-checklist
title: 'Planning: Add internal/encryption crypto primitives (slice 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

This is slice 1 of a six-slice feature (FEAT-JPJ2C). The goal is a
self-contained `internal/encryption/` package with the crypto primitives that
all later slices will use. No rela domain coupling in the core; a thin
`LoadFromDir` helper encodes rela filesystem conventions (env var + default
paths) without importing any rela packages — mirrors how `internal/ai/loader.go`
is structured.

**In scope:**

- Hybrid keypair: X25519 + ML-KEM-768 combined, PEM-encoded with versioned block types
- Data key generation (`NewDataKey`) as the single entropy entry point
- Key wrapping: length-prefixed magic-headered blob (`RLAE` + version + X25519 ephemeral + ML-KEM-768 ciphertext + GCM-wrapped key)
- AEAD: `Seal` / `Open` (stdlib naming) with random 12-byte nonce prepended; documented 2^32 message-per-key ceiling
- Keyring: loads recipients from a directory; optional local private key; exposes `Recipient(id)`, `Identities()`, `Unwrap(blob)` — no internal map leakage
- Rela-convention loader: `LoadFromDir(relaDir)` resolves `$RELA_KEY_FILE` → `<relaDir>/key` → `~/.config/rela/key` without importing rela types
- Typed sentinel errors (`ErrNoPrivateKey`, `ErrBadPEM`, `ErrBadBlob`, `ErrDecrypt`) for `errors.Is`
- Redaction discipline + table-driven leak test (mirrors `ai/redact_test.go`)
- 100% test coverage, deterministic (swappable `randReader`)

**Out of scope (later slices):**

- Metamodel parsing of `encrypted:` declarations
- Group resolution (belongs with metamodel/config at wiring site — NOT here)
- `fsstore` integration (`!enc` YAML tag read/write)
- CLI commands (`rela keys generate`)
- Key version tracking and rotation
- Data-entry / MCP / desktop wiring
- Passphrase-protected private keys (threat model is remote storage, not local FS)
- Encrypted cache / graph (deferred pending store refactor)

**Acceptance Criteria:** (documented in ticket — 13 criteria, each mapped to
tests below)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Libraries considered:

- **SOPS** (Mozilla/CNCF): the reference tool for partial-encryption in YAML/JSON in git repos. Inspired the per-value `!enc` approach planned for later slices, but SOPS itself targets OpenPGP/age/KMS — not PQ. We borrow the *model* (per-value encryption, per-file data key wrapped per recipient, metadata block in the file) without using SOPS as a dependency.
- **age** (FiloSottile): modern file encryption, X25519-based. Proves the "small PEM-ish key format, keyring from filenames" ergonomics. Not PQ-ready; no built-in ML-KEM support. Adopting age directly would lock us to classical crypto.
- **Ansible Vault**: full-file encryption only, no partial; weaker threat model. Not a fit.
- **git-crypt / BlackBox / Transcrypt**: full-file, GPG-based. Lose partial-field encryption and don't diff cleanly.
- **Go stdlib `crypto/mlkem`** (Go 1.24+): NIST FIPS 203 ML-KEM. Fallback: `github.com/cloudflare/circl/kem/mlkem/mlkem768` — widely deployed, audited.
- **Go stdlib `crypto/ecdh`**: X25519 implementation.
- **Signal / Chrome / Apple iMessage**: the hybrid X25519 + ML-KEM-768 pattern. Established reference for the envelope structure.

Patterns in codebase:

- `internal/ai/` — the structural model. Self-contained package, `LoadProvider(relaDir)` entry point that encodes rela conventions without importing rela types. `ai/redact.go` + `ai/redact_test.go` — table-driven leak test. `ai/errors.go` — typed sentinel errors with `errors.Is`. The encryption package mirrors this layout exactly.
- `internal/secrets/` — minimal package scope for comparison (script secrets). Different concern, no reuse.

Concepts reviewed:

- `encryption` (just created) — this package's concept doc
- `ai-integration` — established the "cross-cutting capability, self-contained package, wired at entry points" pattern

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Follow the `internal/ai/` layout exactly. One file per concern; package-level
functions for primitives; a `Keyring` struct that encapsulates the loaded state.

### Crypto construction

- X25519 via `crypto/ecdh`: ephemeral keypair generated per `WrapKey` call.
- ML-KEM-768 via `crypto/mlkem`: encapsulation produces a shared secret + ciphertext.
- HKDF-SHA256 combines both shared secrets (`X25519_ss || MLKEM_ss`) with a named `info` constant (`hkdfInfoV1 = "rela-encryption v1"`) and nil salt into a 32-byte key-encryption key (KEK). Inspired by RFC 9180 §4.1 `LabeledExtract`/`LabeledExpand` but a simplified form, not HPKE proper — godoc calls this out so a future swap bumps the version cleanly.
- KEK is used with AES-256-GCM to wrap the 32-byte data key. Output: 32 bytes ciphertext + 16 bytes GCM tag = 48 bytes.
- Wrapped blob: `RLAE (4B) || v=0x01 (1B) || X25519 ephemeral pub (32B) || ML-KEM ct (1088B) || wrapped (48B)` = 1173 bytes.

### PEM format

Versioned block types so a future algorithm change produces distinguishable
PEMs:

- `RELA X25519-MLKEM768 PRIVATE KEY V1` — serialises X25519 scalar (32B) concatenated with ML-KEM-768 decapsulation key (2400B).
- `RELA X25519-MLKEM768 PUBLIC KEY V1` — X25519 public (32B) + ML-KEM-768 encapsulation key (1184B).
- `RELA WRAPPED KEY V1` — the 1173-byte blob.

### AEAD

`Seal(plaintext, dataKey)`:
1. Generate 12-byte nonce from `randReader`.
2. `aes.NewCipher(dataKey)` → `cipher.NewGCM(block)`.
3. Return `nonce || aead.Seal(nil, nonce, plaintext, nil)`.

`Open(ciphertext, dataKey)`:
1. Reject if `len < 12 + 16` (nonce + tag minimum).
2. Split nonce + rest.
3. `aead.Open(nil, nonce, rest, nil)`; wrap GCM error as `ErrDecrypt`.

### Keyring

`LoadKeyring(keysDir, privateKeyPath)`:
1. Walk `keysDir` non-recursively, load each `*.pub` file via `ParsePublicKeyPEM`. Filename without `.pub` = identity. Non-PEM files → wrapped `ErrBadPEM` naming the file.
2. If `privateKeyPath != ""` and file exists: load via `ParsePrivateKeyPEM`.
3. If `privateKeyPath != ""` and file does not exist: error. (Explicit path should resolve.)
4. Return `Keyring` with recipients map (internal) and optional private keypair.

`LoadFromDir(projectRoot)`:
1. Derive `relaDir := filepath.Join(projectRoot, ".rela")` internally.
2. Resolve private key: `$RELA_KEY_FILE` → `<relaDir>/key` → `~/.config/rela/key`. The first path that is set and exists wins; no hit = "" (no private key).
3. Call `LoadKeyring(<projectRoot>/keys, resolvedPrivateKeyPath)`.
4. API deliberately diverges from `ai.LoadProvider(relaDir)` — encryption has two roots of interest (`.rela/key` and `<projectRoot>/keys/`), so taking `projectRoot` avoids a brittle `filepath.Dir` heuristic. Callers in rela's `project.Context` already have `projectRoot` cleanly.

### Entropy

Stdlib style (mirrors `crypto/ecdsa.SignASN1`): each exported zero-arg function
that needs randomness wraps an unexported helper taking `io.Reader`:

```go
func GenerateKeypair() (*Keypair, error)               { return generateKeypair(rand.Reader) }
func generateKeypair(r io.Reader) (*Keypair, error)    { /* ... */ }
```

Tests call the unexported helpers directly with a fixed reader — no package-global
state, no test-only setter, no `t.Cleanup` choreography.

### Redaction

The shape deliberately diverges from `ai/redact.go`. In `ai`, the threat is
echoing a known secret *string* (the API key) from upstream JSON. Here the
threat is the opposite: errors built from in-memory secret `[]byte` via
`fmt.Errorf("%v", …)`. Two-part discipline:

1. **Type-level hiding**: `Keypair`, `PublicKey`, `DataKey`, and any `[]byte`
   wrapper around scalar/data-key/plaintext/ciphertext define no `String()`,
   `GoString()`, or `MarshalJSON` methods. Where raw `[]byte` must be rendered,
   a helper `safe(b []byte) string` returns `"<N bytes>"`.
2. **Centralised error constructors**: ~6 constructors are the only sites that
   build sentinel-wrapped errors (`errBadBlobAt(offset)`, `errDecryptGCM(cause)`,
   `errBadPEM(filename, cause)`, …). Leak test asserts on these constructors
   directly, with distinctive byte patterns, rather than enumerating every call
   site.

Godoc calls out that `ErrBadBlob` vs `ErrDecrypt` is not a security-sensitive
distinction (the blob format is public). Callers should not expose the
distinction in UX fed by untrusted input.

**Files to modify/create:**

- `internal/encryption/doc.go` — package docstring, threat model
- `internal/encryption/keypair.go` — `Keypair`, `PublicKey`, `GenerateKeypair`
- `internal/encryption/pem.go` — `MarshalPrivateKeyPEM`, `ParsePrivateKeyPEM`, `MarshalPublicKeyPEM`, `ParsePublicKeyPEM`
- `internal/encryption/wrap.go` — `WrapKey`, `UnwrapKey`, blob format constants
- `internal/encryption/aead.go` — `Seal`, `Open`
- `internal/encryption/datakey.go` — `NewDataKey`, `DataKeySize`, `randReader`
- `internal/encryption/keyring.go` — `Keyring`, `LoadKeyring`
- `internal/encryption/loader.go` — `LoadFromDir`
- `internal/encryption/errors.go` — sentinels
- `internal/encryption/redact.go` — redaction helpers
- Matching `*_test.go` files for each

Also:

- `go.mod` — add `github.com/cloudflare/circl` if `crypto/mlkem` is unavailable on the project's Go version (check `go version` in toolchain first)
- `.testcoverage.yml` — add 100% floor override for `internal/encryption/`

**Alternatives considered:**

- **Age as a dependency**: rejected. Locks us to classical crypto; no clean PQ extension path.
- **Pure X25519 (no ML-KEM)**: rejected. Quantum adversary would recover wrapped keys.
- **Pure ML-KEM (no X25519)**: rejected. ML-KEM is newer; hybrid hedges against PQ algorithm flaws.
- **Interfaces over `Keypair`/`PublicKey`**: rejected (YAGNI). Single algorithm; stdlib pattern is concrete types, accept interfaces at consumers.
- **Methods instead of package-level `WrapKey`/`UnwrapKey`**: rejected. The operation is joint between keys; stdlib (`ecdh.Sign`, etc.) uses free functions; symmetric naming reads better.
- **`ResolveGroup` in this package**: rejected (removed after architect review). Groups are a metamodel concept; crypto primitives shouldn't know.
- **`Keyring.PrivateKey() *Keypair` exposure**: rejected. Least-privilege: `Unwrap(blob)` is all callers need.
- **Fixed-size blob (no magic/version)**: rejected. Self-describing format survives copy-paste, enables clean rejection of future format changes.
- **`EncryptValue`/`DecryptValue` naming**: rejected in favour of `Seal`/`Open` (stdlib `crypto/cipher.AEAD` pattern).

**Dependencies:**

- `crypto/rand`, `crypto/cipher`, `crypto/aes`, `crypto/ecdh`, `crypto/mlkem` (stdlib)
- `crypto/sha256`, `golang.org/x/crypto/hkdf` (likely already in go.sum via transitive)
- `encoding/pem` (stdlib)
- `github.com/cloudflare/circl` (fallback only, pinned version)
- No rela domain imports

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | Invalid handling |
|---|---|---|
| Recipient PEM files in `keysDir` | PEM block type must match `RELA X25519-MLKEM768 PUBLIC KEY V1` exactly; payload length must match expected (32 + 1184 bytes) | Wrap `ErrBadPEM` with filename |
| Private key PEM at resolved path | PEM block type `RELA X25519-MLKEM768 PRIVATE KEY V1`; payload length exact | Wrap `ErrBadPEM` |
| Wrapped blob for `UnwrapKey` | Magic `RLAE`, version byte, total length 1173 | Return `ErrBadBlob` |
| Ciphertext for `Open` | Minimum length (12 + 16 bytes) | Return `ErrDecrypt` |
| Data key for `WrapKey`/`Seal` | Length must be exactly 32 | Programmer error; return wrapped sentinel |
| `$RELA_KEY_FILE` env var | Treated as opaque path; no shell expansion; `os.Open` directly | If set but file missing, error (explicit opt-in should resolve) |

All length/format checks use **allowlist** (exact match against expected shape)
rather than denylist.

**Security-Sensitive Operations:**

- **Private key loading**: read with `os.ReadFile`; never logged, never embedded in errors; zeroed (best-effort) after parsing into scalars.
- **KEK derivation via HKDF**: single deterministic path; no user-supplied salt.
- **Nonce generation**: exclusively via `randReader`; never exposed to callers; never reused (random 12 bytes per `Seal` call).
- **GCM authentication**: `Open` returns `ErrDecrypt` wrapping (not reflecting) the GCM failure — no oracle leak.
- **Error messages**: every construction path goes through `redact.go` helpers. Leak test asserts no key/plaintext/ciphertext bytes appear in any error string.
- **Zeroing**: best-effort after use, documented as best-effort (GC may have moved memory). Not a security guarantee.

**Threat model boundaries** (documented in `doc.go`):

- Protects: at-rest data in shared git repos, cloud storage, backups, forks
- Does NOT protect: in-memory data, file names, entity IDs, metamodel structure, relation topology
- Does NOT provide: forward secrecy, passphrase-gated private keys (slice 1)
- Does NOT scrub git history on key rotation

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (criterion → test):**

1. Keygen/PEM round-trip → `TestKeypair_PEM_RoundTrip`: generate → marshal → parse → marshal; compare bytes equal
2. Wrap/unwrap round-trip → `TestWrap_RoundTrip`: random data key; wrap for pub; unwrap with priv; bytes equal
3. Cross-key failure → `TestUnwrap_WrongKey`: generate two keypairs; wrap for A; unwrap with B → `ErrDecrypt`
4. Seal/Open round-trip → `TestAEAD_RoundTrip`: various plaintext sizes (0, 1, 15, 16, 17, 1024)
5. Tamper detection → `TestAEAD_Tamper`, `TestWrap_Tamper`: flip each byte in the first N positions; expect sentinel error
6. Blob magic/version/length rejection → `TestWrap_BadBlob` table
7. PEM block type/version rejection → `TestPEM_BadType` table
8. Keyring load → `TestLoadKeyring`: temp dir with mix of valid/invalid files
9. `LoadFromDir` precedence → `TestLoadFromDir_Precedence` table: env set, project-local, user default, all missing, explicit-env-missing
10. `Keyring.Recipient`/`Identities`/`Unwrap` → `TestKeyring_Accessors`
11. Leak test → `TestRedaction_NoLeaks`: table over the centralised error constructors with distinctive bytes; also asserts secret types have no `String`/`GoString`/`MarshalJSON` methods
12. Data key determinism → `TestDataKey_Deterministic`: calls unexported helper with a fixed reader
13. Multi-recipient composition → `TestWrap_MultipleRecipients`: wrap the same data key for two keypairs; each unwraps to the original
14. Coverage → `just test-coverage`; CI enforces via `.testcoverage.yml` floor

**Edge Cases:**

- Empty plaintext (0 bytes) — Seal/Open should succeed
- Maximum practical plaintext (e.g. 1 MiB) — check we don't allocate quadratically
- PEM with extra whitespace, comments, CRLF line endings
- Keys directory with `.pub` files in subdirectories (ignored — non-recursive)
- Keys directory with hidden files (e.g., `.DS_Store`) — skipped by extension check
- Keys directory that doesn't exist — treat as empty recipients (clear error only if later Wrap references unknown identity)
- Private key path pointing to directory (not file) — `ErrBadPEM` or wrapped IO error
- Unicode in filenames — identity is raw filename minus `.pub`; no normalisation
- Wrapped blob of exactly min/max-1/max+1 length
- Data key of wrong length (16 bytes, 64 bytes) — programming error; return wrapped sentinel

**Negative Tests:**

- Parse: empty bytes, non-PEM bytes, wrong block type, right block type but wrong version suffix, truncated payload, payload of wrong length
- Wrap: nil recipient, wrong-length data key
- Unwrap: bytes that pass magic check but fail GCM — `ErrDecrypt`; wrong-magic bytes — `ErrBadBlob`
- Seal: nil key, wrong-length key
- Open: short ciphertext (< 28 bytes), all-zero ciphertext — `ErrDecrypt`
- Keyring: missing `keysDir`; unreadable file in `keysDir`; mixed valid and invalid PEM files (errors aggregate or fail fast — pick fail-fast for now)
- LoadFromDir: env set but file missing; all paths unset (no private key — not an error)

**Integration test approach:**

Even in slice 1, an end-to-end test exercises the full flow: generate keypair →
marshal PEM → write to temp dir as `keys/alice.pub` + `~/.config/rela/key` style
layout → `LoadFromDir` → `NewDataKey` → `WrapKey` using
`keyring.Recipient("alice")` → `Seal` → `Open` via a separately-loaded keyring
with the matching private key → verify plaintext matches. This is a single Go
test (no external process), but it proves all pieces compose correctly.

Later slices will add cross-package integration (store reads/writes an encrypted
file, decodes it). For slice 1 the in-package end-to-end is sufficient.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Nonce reuse in AES-GCM** (catastrophic): mitigated by random 12-byte nonce per Seal; documented 2^32-per-key ceiling; contract that each file uses a fresh data key.
- **`crypto/mlkem` availability**: need Go 1.24+. Mitigation: detect at compile time or fall back to `circl`. Check the project toolchain first — if 1.24 isn't pinned, use `circl` directly for predictability.
- **HKDF misuse**: combine `X25519_ss || MLKEM_ss` + fixed context string + salt = `nil` or fixed. Follow RFC 9180 / hybrid PKE guidance. Not inventing — use a widely-reviewed construction.
- **Entropy failure**: `rand.Reader` failure produces an error. Never silently degrade; always propagate.
- **Key zeroing ineffective**: documented as best-effort; not a security guarantee.
- **Effort**: `m` (1–2 days). Crypto itself is stdlib/circl calls; bulk of time is tests (100% coverage) and redaction discipline.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A for slice 1 — pure library with no user-facing surface yet
- [x] ~~CLAUDE.md update~~ (N/A: deferred to the slice that lands `rela keys generate` and metamodel config)
- [x] ~~User guide for encryption setup~~ (N/A: deferred to slice 6)

No docs-checklist needed for this slice (no user-facing changes). Will be
created for slices that touch CLI or data entry UI.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Reviewed with the `go-architect` agent twice before transitioning to in-progress.
All feedback incorporated into the ticket and this plan; no open review-response
entities — findings were addressed as plan changes.

First pass:

- Dropped `ResolveGroup` (wrong package boundary)
- Dropped `Keyring.PrivateKey()` (over-exposure) in favour of `Keyring.Unwrap(blob)`
- Renamed `EncryptValue`/`DecryptValue` → `Seal`/`Open` (stdlib naming)
- Split `LoadKeyring` (pure) from `LoadFromDir` (rela conventions)
- Made PEM functions symmetric package-level (not mixed methods + functions)
- Added `NewDataKey` to centralise entropy
- Added typed sentinel errors for `errors.Is`
- Magic-headered wrapped blob format
- Versioned PEM block types (`X25519-MLKEM768 ... V1`)
- Raised coverage target from 85% to 100%
- Added `redact.go` + leak test from the start

Second pass (independent review):

- **S1**: `LoadFromDir(relaDir)` → `LoadFromDir(projectRoot)`; derive `.rela` internally. Drops the brittle `filepath.Dir` conditional.
- **S2**: Redaction shape diverges from `ai/redact.go` — type-level hiding (no `String()`/`GoString()`/`MarshalJSON` on secret types) + centralised error constructors, not a `redactKey`-shaped helper. Different threat.
- **M1**: `randReader` package var → unexported reader-taking helpers (`generateKeypair(r io.Reader)`) wrapped by zero-arg exported functions. Stdlib style; no test-only global.
- **M4**: Document in godoc that `ErrBadBlob` vs `ErrDecrypt` is not a security-sensitive distinction.
- **M5**: Name the HKDF info constant (`hkdfInfoV1`); cite RFC 9180 as inspiration, not compliance.
- **N4**: Add two-recipient end-to-end test — proves the primitive composes for slice 2+ without a later shape change.

Confirmed unchanged (pushback dismissed):

- Sentinel errors (not `*Error{Kind}`) — crypto errors are terminal, no retry taxonomy
- PEM versioning in block type — `pem.Decode` rejects at the boundary
- Fixed-size blob, no length prefixes — every V1 component is fixed-size; doc this so no one "helpfully" adds them
- 10 files — one concern per file, stdlib style
- Zeroing — keep as internal best-effort, not documented as a public guarantee
