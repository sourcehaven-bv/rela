---
id: TKT-R25H3
type: ticket
title: 'fsstore integration: transparent !enc read/write (slice 3)'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Summary

Slice 3 of FEAT-JPJ2C: wire the `internal/encryption/` primitives (slice 1) and
the metamodel `encrypted:` / groups metadata (slice 2) into
`internal/store/fsstore/` so entity read/write transparently encrypts and
decrypts on disk.

After this slice lands, a user running `rela` against a project with:
- `encrypted: engineering` on a property,
- `groups.yaml` listing identities,
- `keys/*.pub` for those identities,

will see that property stored under a `_enc_v1_<name>:` key in the YAML
frontmatter and decrypted in memory — provided they have a private key that
matches a listed recipient. Without a matching key, the property surfaces as an
`encryption.Opaque` sentinel value (not garbage, not original cleartext); other
properties still load correctly.

## Wire format decision: key-prefix, not YAML tag

The concept doc originally sketched a YAML `!enc` tag. During slice 3 planning
we discovered that `yaml.v3`'s `Unmarshaler` interface is **not invoked** when
the destination is `map[string]interface{}` (which is what
`fsstore.parseDocument` uses). That would require a substantive parser migration
to `*yaml.Node` walking.

After considering three alternatives — YAML tag (parser migration required),
value-prefix (`rela-enc:v1:...`), and key-prefix (`_enc_v1_<name>:`) — we picked
**key-prefix**. Comparison:

| Approach | Parser work | Collision risk | grep-able |
|---|---|---|---|
| `!enc` YAML tag | High (yaml.Node migration) | None | No |
| Value prefix | None | Near-zero with specific string | Yes |
| **Key prefix `_enc_v1_<name>`** | None | None (reserved `_` namespace) | Yes |

Key-prefix wins on three fronts: no parser changes, zero collision risk (the `_`
prefix is already reserved for fsstore metadata like `_encryption`), and `grep
-r '_enc_v1_' entities/` finds encrypted files instantly. The concept doc will
be updated to match.

## On-disk format (revised)

```yaml
---
id: TKT-123
type: ticket
title: Fix auth bug              # cleartext
status: open                      # cleartext
_enc_v1_description: SGVsbG8gd29ybGQgY2lwaGVydGV4dA==
_enc_v1_notes: QW5vdGhlciBzZWNyZXQ=
_encryption:
  key_version: 1
  data_keys:
    engineering:
      alice: a3VHdHdwZnN5cWJjcHgwMW1kZzE2bnR3...
      bob:   b2VnM2VwZ2RuY3hmb2s0cG9wZDg0czU5...
---

# Fix auth bug

(body content — cleartext here, but see encrypted_body below)
```

- `_enc_v1_<propName>` keys hold base64(nonce || ciphertext || GCM tag) of the
encrypted property value. The `v1` is the on-disk format version.
- `_encryption.data_keys` maps `<group> → <identity> → wrapped-data-key` (base64).
- Properties not encrypted continue to use their plain names (`title`, `status`).
- Property order in the frontmatter preserves the metamodel-defined order; an
encrypted property's slot stays put, only its key is rewritten.

Encrypted bodies (`encrypted_body:` on `EntityDef`) need a separate container
since the markdown body isn't a property. They become a second YAML document or
a `_encrypted_body:` frontmatter key holding base64 ciphertext, with the literal
body empty. Leaning toward `_encrypted_body:` for simplicity.

## Scope

### In scope

- **Key-prefix rename on read/write**: `readEntityFile` strips `_enc_v1_` prefix + decrypts value; `writeEntityFile` applies prefix + encrypts value for properties declared `encrypted:` in the metamodel.
- **Per-file data key**: each write generates a fresh `NewDataKey()`; the data key wraps per recipient in the `_encryption` envelope; encrypted values use that data key for AES-GCM `Seal`. One data key per file (across all groups referenced in that file — each group gets its own wrapped copy).
- **`_encryption` frontmatter block**: structured as above. `key_version` refers to the envelope format version (currently 1; room to evolve).
- **Single `Crypto` interface on `FSStore.Config`**: per architect review S1, collapse the two planned fields (`Keyring`, `Groups`) into one consumer-defined interface. nil = cleartext-only store.
- **Keep `EntityTypeSchema` untouched**: per architect review S2, encryption policy lives on the `Crypto` interface (`PropertyGroup(type, prop)`, `BodyGroup(type)`), not on the per-type schema. This also avoids a `fsstore → metamodel` arch-lint edge.
- **`encryption.Opaque` sentinel type**: a distinct type in `internal/encryption/` representing "value was encrypted but we couldn't decrypt." Holds the raw wire-format ciphertext bytes so it can be re-emitted verbatim on write. `String()` returns `"<encrypted>"`; `MarshalJSON` ditto.
- **Partial-decrypt semantics**: if the local keyring has no matching private key for a given group, affected properties come back as `Opaque`. Entity loads, other properties intact. Writes are refused while any `Opaque` value is present (no re-sealing data we can't read).
- **Reserved-name validation (metamodel package)**: a single rule in `internal/metamodel/validation.go` rejects any property name matching `^_.*`. Covers `_encryption`, `_enc_v1_*`, and any future reserved key uniformly. Lives at metamodel-load time so fsstore only ever operates on a validated schema — attackers can't "slip through" a malformed metamodel to corrupt files.
- **Typed errors**: new `fsstore.EncryptionError{Kind, Property, Cause}` wrapping `encryption` package sentinels. Kinds: `MissingKeyring`, `WrongKey`, `OpaqueWrite`, `CorruptedFile`.
- **Integration tests**: round-trip through fsstore; multi-recipient; partial-decrypt (wrong key loaded); tamper detection; backward-compatibility (entities without `encrypted:` declarations still work); encrypted body.

### Out of scope (deferred)

- Key versioning / rotation logic — slice 4
- CLI (`rela keys generate`, `rela keys rotate`) — slice 5
- Data-entry UI / MCP surface — slice 6
- Re-encrypting on metamodel changes (adding `encrypted:` to existing cleartext field) — slice 4's "lazy re-encryption"
- Cache / in-memory graph — cache.json remains plaintext (known limitation from concept doc)

## Design Sketch (revised per architect review)

```go
// internal/store/fsstore/encryption.go (new) — consumer-defined interface
package fsstore

// Crypto is the subset of encryption capability fsstore needs. A nil
// *Config.Crypto preserves today's cleartext-only behaviour exactly.
type Crypto interface {
    // PropertyGroup returns the group for which the given property of
    // the given entity type should be encrypted. (_, false) = cleartext.
    PropertyGroup(entityType, property string) (group string, encrypted bool)

    // BodyGroup returns the group for which the entity's markdown body
    // should be encrypted. (_, false) = cleartext body.
    BodyGroup(entityType string) (group string, encrypted bool)

    // Recipients returns the ordered identity list for the group.
    Recipients(group string) ([]string, bool)

    // Recipient returns the public key for an identity; used at wrap time.
    Recipient(identity string) (*encryption.PublicKey, bool)

    // UnwrapAny takes a group's full {identity → wrapped-data-key} map
    // and returns the first entry the local keyring can decrypt, along
    // with the identity that matched. Returns (nil, "", ErrNoMatchingKey)
    // when no identity matches (→ Opaque), or (nil, "", ErrDecrypt /
    // ErrBadBlob) when a match was attempted but the blob was corrupt
    // (→ CorruptedFile). This keeps fsstore free of walk-order logic
    // and lets the error taxonomy distinguish partial-decrypt from
    // corruption — see criteria 3 and 7.
    UnwrapAny(wraps map[string][]byte) (dataKey []byte, matched string, err error)

    // HasPrivateKey reports whether the local keyring can decrypt anything.
    // Used to distinguish "no key configured" vs "wrong key."
    HasPrivateKey() bool
}

// internal/store/fsstore/fsstore.go — Config extension
type Config struct {
    // ... existing fields ...
    Crypto Crypto // nil = cleartext-only
}

// internal/encryption/opaque.go (new, in encryption package)
// Opaque marks an encrypted value the current keyring couldn't decrypt.
// Round-trips through map[string]interface{}; prints as "<encrypted>";
// preserves raw wire-format bytes so unmodified opaque values can be
// re-written verbatim (they re-seal under an unchanged envelope).
type Opaque struct {
    ciphertext []byte // private: copy on construction, no mutation possible
}
func NewOpaque(cipher []byte) Opaque { /* defensive copy */ }
func (Opaque) String() string { return "<encrypted>" }
func (o Opaque) MarshalJSON() ([]byte, error) { return []byte(`"<encrypted>"`), nil }
func (o Opaque) Bytes() []byte { /* defensive copy — public API */ }

// BorrowBytes returns the underlying ciphertext WITHOUT copying. The
// caller MUST NOT mutate the returned slice. Use Bytes() if you need
// an independent copy. Documented as a performance escape hatch for
// re-emit-on-write.
func (o Opaque) BorrowBytes() []byte { return o.ciphertext }

// internal/app/factory.go — the adapter assembly site
type cryptoAdapter struct {
    mm *metamodel.Metamodel
    g  *metamodel.Groups
    kr *encryption.Keyring
}
// implements fsstore.Crypto by combining the three
```

### Key-prefix helpers + seal/unseal

```go
// internal/store/fsstore/encmarshal.go (new)
const (
    encKeyPrefix       = "_enc_v1_"
    encryptionKey      = "_encryption"
    encryptedBodyKey   = "_encrypted_body"
)

// stripEncKey returns (propName, true) if key is an _enc_v1_ key.
func stripEncKey(key string) (string, bool) {
    rest, ok := strings.CutPrefix(key, encKeyPrefix)
    return rest, ok
}

// applyEncKey wraps a plain property name with the prefix.
func applyEncKey(propName string) string { return encKeyPrefix + propName }
```

**Shape A** for property-order substitution (per design review): `formatEntity`
stays oblivious to encryption. fsstore calls a new `sealProperties` *before*
calling `formatEntity`; it receives an already-rewritten map and key order.

```go
// sealProperties translates an entity's plain property map to its wire-format
// shape: encrypted property names get the _enc_v1_ prefix, their values become
// base64 ciphertext strings, and an _encryption frontmatter block is injected.
// Returns (frontmatterMap, rewrittenKeyOrder, err).
//
// Returns (original map, original order, nil) when s.crypto is nil (fast path).
func (s *FSStore) sealProperties(e *entity.Entity, order []string) (map[string]any, []string, error)

// unsealProperties is the inverse: walks a just-parsed frontmatter, decrypts
// _enc_v1_* entries back to their plain property names, removes the
// _encryption block from the returned map. Values that can't be decrypted
// become encryption.Opaque sentinels.
func (s *FSStore) unsealProperties(fm map[string]any) (map[string]any, error)
```

Keeps the crypto layer off `formatDocumentOrdered` entirely. The formatter
continues to process `map[string]any` keyed by whatever names it's given.

## Acceptance Criteria

1. Write an entity with an `encrypted: engineering` property; file on disk shows `_enc_v1_<name>: <base64>` plus a `_encryption:` frontmatter block listing the group and recipients.
2. Read that entity with a keyring holding a matching private key → property decrypted into memory under its original (non-prefixed) name, with its original value.
3. Read the same entity with a keyring holding a non-matching private key → property present under original name but holds an `encryption.Opaque` value; other properties load correctly.
4. Read without any keyring configured → `fsstore.EncryptionError{Kind: MissingKeyring}` propagates to the caller.
5. Unchanged `Opaque` values are preserved verbatim on write — re-emitted byte-for-byte under the same `_enc_v1_*` key, with the envelope's wrapped data keys untouched. This lets a user who can't decrypt engineering-group fields still edit cleartext properties without touching the engineering ciphertext. Write is refused with `fsstore.EncryptionError{Kind: OpaqueWrite, Property: <name>}` only when the caller (a) has replaced the `Opaque` with a different value (no data key to re-seal), or (b) an `Opaque` appears at write time under a property whose metamodel group no longer matches the envelope's group.
6. Multi-recipient: entity encrypted for `engineering: [alice, bob]` reads cleanly with either's keyring.
7. Tamper: flipping any byte in a `_enc_v1_*` value or in a `_encryption.data_keys` wrapped blob causes read to return `fsstore.EncryptionError{Kind: CorruptedFile}` wrapping the underlying `encryption.ErrDecrypt`/`ErrBadBlob`. Distinguishable at the typed-error level from partial-decrypt (criterion 3).
8. Backward-compat: entities without any `encrypted:` declarations round-trip byte-identically to today. All existing `fsstore_test` tests still pass unchanged.
9. Encrypted body: an entity type with `encrypted_body: exec` writes the body as `_encrypted_body: <base64>` with an empty markdown body section; reads back to the original content when decryptable.
10. Property order stability: encrypted properties' slots in the YAML preserve metamodel-defined order. Outside the `_encryption.data_keys` block and the `_enc_v1_*` ciphertext values (both of which contain fresh entropy per write), two consecutive writes of an unchanged entity produce byte-identical YAML.
11. Reserved-name validation in the **metamodel** package (not fsstore): a single rule rejects any user property name matching `^_.*`. Runs at metamodel-load time, before fsstore sees the schema. Covers `_encryption`, `_enc_v1_*`, and future reserved keys uniformly.
12. Mixed-group entity: one file can hold properties encrypted for multiple groups (e.g., `description: engineering` + `secret: exec`). A single per-file data key is generated; `_encryption.data_keys` contains one section per referenced group, each wrapping the same data key for that group's recipients.
13. Empty-string round-trip: a property whose value is `""` (encrypted) reads back as `""` and is distinguishable from a missing property after decrypt.
14. `Crypto == nil` fast path: `writeEntityFile` and `readEntityFile` take a single guarded branch that skips all encryption helpers when `cfg.Crypto == nil`. No encryption code runs on cleartext-only stores.
15. Body-key conflict rejection: a write where `_encrypted_body` is set AND the in-memory `entity.Content` is non-empty is rejected as `fsstore.EncryptionError{Kind: BodyConflict}`. A first write with `encrypted_body:` on an entity that currently has cleartext `Content` seals the content (the normal encrypt path); subsequent writes that mutate `Content` re-seal. This defends against accidental co-storage of ciphertext and cleartext under the same semantic slot.
16. Observer contract: `store.EntityObserver` callbacks (notifyPut) receive entity `Properties` as loaded — `Opaque` values flow through unchanged. Observers (propCache, graph sync, MCP watcher) treat `Opaque` as an opaque filterable scalar via `String()` / `MarshalJSON`. No observer signature changes.
17. `go-arch-lint` clean — `fsstore → encryption` added to allowed deps. `fsstore → metamodel` **not** added (consumer-defined interface keeps metamodel out).
18. Coverage: new code (encmarshal.go, encryption.go, opaque.go, modified markdown.go branches) ≥ 85%.

## Test Plan

**Unit (encryption/opaque_test.go):**
- `NewOpaque` defensively copies input; mutation of input doesn't affect `Opaque.Bytes()`.
- `String()` returns `"<encrypted>"`.
- `MarshalJSON()` produces `"<encrypted>"`.
- Opaque is comparable by content bytes (for test assertions).

**Unit (fsstore/encmarshal_test.go):**
- `stripEncKey("_enc_v1_description")` → `("description", true)`.
- `stripEncKey("description")` → `("", false)`.
- `applyEncKey("description")` → `"_enc_v1_description"`.
- Round-trip: apply then strip recovers input.

**Unit (fsstore/encryption_test.go — new):**
- Given a fake `Crypto`, encrypt one property → the resulting map has `_enc_v1_*` key and a `_encryption` block. Decrypt recovers original value.
- Multi-recipient: wrap for two identities → decrypt with either keyring works.
- Cross-key (wrap for alice, decrypt with bob) → `Opaque` result.
- Tamper byte in the ciphertext → `CorruptedFile` wrapping `ErrDecrypt`.

**Integration (fsstore round-trip, new encryption_test.go in fsstore_test):**
- Happy path: metamodel with `encrypted: engineering`, `Crypto` adapter with alice's keyring; create → read → update → delete.
- Wrong key: swap to eve's private key; read yields Opaque values; write attempt refused.
- Multi-group on same entity: two properties, two different groups, partial decrypt works.
- Body encryption: `encrypted_body: exec`, round-trip.
- Cleartext-only: existing `persistence_test.go` suite runs with `Crypto: nil`, all tests pass.
- Determinism: write twice with identical inputs (fresh data key each write) → `_enc_v1_*` values differ (entropy check), `data_keys` entries differ; non-encrypted fields and structure byte-identical.

## Dependencies

- TKT-16RY1 (slice 1) — crypto primitives (merged on this branch)
- TKT-OGLXI (slice 2) — metamodel `encrypted:` parsing + `Groups` loader (merged on this branch)

## Risk Assessment

- **Medium**: this is the integration slice. Wire format is now simpler (no YAML tag) but correctness of encrypt/decrypt + backward-compat is still the main risk.
- **Security risk**: incorrect key-renaming could leak plaintext under its original key. Mitigation: exhaustive round-trip tests; leak test asserts no cleartext bytes of the test plaintext appear in the on-disk file (other than via legitimate reuse — e.g., if the plaintext happens to be `"open"` and another unencrypted field is `"open"`, that's fine).
- **Data-loss risk**: writing while in Opaque state corrupts. Mitigation: acceptance criterion 5 refusal enforced at write time.
- **Integration risk**: existing fsstore tests might break. Mitigation: Crypto=nil preserves exact current behaviour; run the full existing fsstore suite green before adding encryption tests.
- **Property-cache risk**: `FSStore.propCache` tracks value-frequency for filter UIs. Opaque values all stringify to `"<encrypted>"` and collapse into one bucket. That's the right behaviour — don't expose encryption state as filterable data — but worth explicit test.

## Effort

**m-l** — 1.5 to 2 days. Simpler than the original estimate because:
- No `yaml.Node` migration (key-prefix bypasses the `Unmarshaler` pitfall entirely)
- `EntityTypeSchema` unchanged (consumer-defined `Crypto` interface)
- `fsstore → metamodel` arch edge avoided

Most time is in the integration tests and the `cryptoAdapter` wiring in
`internal/app/factory.go`.

## Implementation ordering (per pass-3 review)

Landing as three verifiable steps keeps reviewer cognitive load bounded and each
commit passes CI independently:

**Step 1 — foundations** (standalone):
- `encryption.Opaque` type + tests (`internal/encryption/opaque.go`)
- `entity.CloneValue` case for `Opaque` (preserve ciphertext, no deep copy)
- Reserved-name validation in `internal/metamodel/validation.go` (criterion 11)

**Step 2 — sealed helpers** (pure functions, no fsstore wiring):
- `Crypto` interface + `encryption.ErrNoMatchingKey` sentinel
- `sealProperties` / `unsealProperties` as standalone functions in
  `internal/store/fsstore/encmarshal.go`
- Fake `Crypto` implementation for unit tests
- Full round-trip + multi-recipient + mixed-group coverage at the helper level
  before any fsstore integration happens

**Step 3 — wire into fsstore** (integration):
- `Config.Crypto` field + `Crypto == nil` fast path in `readEntityFile` /
  `writeEntityFile`
- `app.FSFactory` builds the `cryptoAdapter` from metamodel + groups + keyring
- Integration tests in `fsstore_test/encryption_test.go`

## Design review history

Three passes with `go-architect`:

- **Pass 1** (original plan with `!enc` YAML tag): five significant findings —
  collapse `Keyring`/`Groups` into a `Crypto` interface (S1), keep
  `EntityTypeSchema` untouched (S2), restrict arch-lint edges (S3), YAML tag
  unmarshalling doesn't work against `map[string]interface{}` (S4), locate
  opaque-write refusal in fsstore (S5). All incorporated.

- **Pass 2** (key-prefix wire format): three significant refinements —
  reserved-name validation must live in metamodel using `^_.*` (criterion 11),
  property-order substitution shape A keeps crypto off the formatter,
  opaque-write refusal must be narrower (criterion 5). Plus minors for
  `BorrowBytes`, mixed-group test, empty-string round-trip, `Crypto == nil` fast
  path. All incorporated.

- **Pass 3** (final pre-implementation check): three significant —
  `Crypto.Unwrap(wrapped)` is insufficient for distinguishing partial-decrypt
  from corruption; upgraded to `UnwrapAny(wraps) (dk, matched, err)` with a new
  `ErrNoMatchingKey` sentinel. Observer contract with `Opaque` values pinned as
  criterion 16. `_encrypted_body` + non-empty body conflict explicitly rejected
  as criterion 15. Plus implementation-ordering guidance (above). All
  incorporated.

  Considered but not adopted: `Recipient` returning a closure to hide
  `encryption.PublicKey` — kept the type dep since `fsstore → encryption` edge
  is already approved and the closure would be cleverness for its own sake.

## Out-of-band concerns flagged for follow-up

- **`entity.CloneValue`** needs a case for `Opaque` (preserves the ciphertext; doesn't mutate). Add in this slice or a tiny precursor.
- **`FormatEntity` (canonical reformatter)** produces byte-churn on every call because data keys rotate. Document: callers that run `rela fmt` on encrypted repos will see per-file diffs even with no logical change. Slice 4 may revisit.
- **External-watcher self-echo hashing**: already content-based, so fresh data keys mean two machines writing the same logical entity independently will not hash-match. Not a slice-3 problem but note in concept doc.
