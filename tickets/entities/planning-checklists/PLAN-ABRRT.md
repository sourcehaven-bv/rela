---
id: PLAN-ABRRT
type: planning-checklist
title: 'Planning: Detect git-crypt encrypted files at fsstore and show inaccessible placeholders in data-entry'
status: done
---

## Understanding

**Problem.** Some users keep parts of a rela project encrypted at rest with
[git-crypt](https://github.com/AGWA/git-crypt). git-crypt is configured per-path
via `.gitattributes`, so partial encryption is the norm: typically
`metamodel.yaml`, templates, and most entities are cleartext, while a subset
(e.g. `entities/secret/**`) is encrypted. When a collaborator clones the repo
without the key (or fails to run `git-crypt unlock`), the encrypted
entity/relation files on disk are raw ciphertext starting with the 9-byte magic
header `\0GITCRYPT\0`.

Today rela has no awareness of this. The markdown parser does not crash on the
binary blob — `bufio.Scanner` in `splitFrontmatter` tolerates NULs — so an
encrypted entity file silently parses to a zero-ID, zero-properties document and
gets indexed (or, worse, overwrites a legitimate entry with empty fields). The
data-entry UI shows nothing useful, the user has no clue git-crypt is the cause.

**Goal.** Detect the git-crypt magic header at the fsstore I/O boundary, surface
"this property exists but is unreadable" as a first-class concept on
`entity.Entity` itself (`Inaccessible []InaccessibleField`), so every consumer —
search, validator, tracer, data-entry — handles partial readability uniformly.
The data-entry SPA shows lock indicators next to each inaccessible property; for
whole-file git-crypt encryption the entity has every schema-declared property
listed as inaccessible.

**Forward compatibility.** The same `Inaccessible` shape will later cover
SOPS-style field-level encryption inside YAML and Lua-driven ACL field
redaction. This ticket implements only the git-crypt whole-file case, but
commits to a model that scales.

## Acceptance criteria

1. **Detection at fsstore.** A new helper `isGitCryptEncrypted(b []byte) bool` returns true iff the first 9 bytes equal `\x00GITCRYPT\x00`. The check is invoked at the lowest common point (`readDataFile`) so every read path — `readEntityFile`, `readRelationFile`, and the watcher's `parseEntityFromPath` — gets coverage with one insertion.

2. **`entity.Entity` carries inaccessibility.** `Entity` gains a field `Inaccessible []InaccessibleField`. Each `InaccessibleField` has a `Name` (property name) and a `Reason` (typed enum, value `InaccessibleReasonGitCrypt` for v1). Invariants: a property name appears in `Properties` **or** `Inaccessible`, never both. Empty `Inaccessible` slice means fully readable (the common case).

3. **Whole-file git-crypt produces a fully-redacted Entity.** When `readEntityFile` detects an encrypted file, the loader returns a `*entity.Entity` with: ID + Type derived from filename/dirname, empty `Properties`, empty `Content`, and `Inaccessible` listing **every property declared by the entity type's metamodel schema** with `Reason: InaccessibleReasonGitCrypt`. The entity is **not** an error — it loads successfully into the index.

4. **Relations.** When `readRelationFile` detects an encrypted file, the loader returns an `*entity.Relation` with: From/Type/To derived from filename, no Properties, no Content, and a parallel `Inaccessible` field marking properties as inaccessible. (Confirm `entity.Relation` shape; if its property surface is small, may inline the marker.)

5. **List view (data-entry).** The list endpoint returns these entities with their full ID/type and an `inaccessible` array on the wire. The SPA renders rows for inaccessible entities with a 🔒 indicator next to each locked property/column. For a fully-inaccessible entity, the row shows the ID, type, and a 🔒 against every column.

6. **Detail view (data-entry).** The detail endpoint returns the entity normally with the `inaccessible` array. The SPA renders the form: each schema property either shows its value (if in `Properties`) or a 🔒 placeholder with reason text (if in `Inaccessible`). For a fully-encrypted entity this means every form field is locked. A help affordance (in-app help link) explains git-crypt unlock.

7. **Save-path safety.** PATCH preserves any inaccessible field's on-disk value: the handler loads the on-disk entity, applies posted fields **only for properties not in `on_disk.Inaccessible`**, and writes. If the client posts a value for a field listed as inaccessible, the handler logs + ignores (does not error 4xx — graceful for legitimate clients that didn't get the latest state). On the wire, the SPA does not submit values for locked fields. Critically: under no circumstance does a client write replace an inaccessible field's value with anything (including empty/null).

8. **Re-read on PATCH for staleness.** Before writing, PATCH re-reads the file and re-detects git-crypt status. If the file is now inaccessible (e.g. file was decrypted, edited in SPA, then re-encrypted), reject with a clear error. mtime/etag check is out of scope for this ticket; explicitly noted.

9. **Validator + iterator consumers degrade gracefully.** `internal/validator/validator.go` and `internal/search/index.go` and `internal/lua/runtime.go:luaListEntities` consume `ListEntities`. Now that `ErrEncrypted` is **not** an error case (the entity loads with `Inaccessible` populated), no per-consumer change is needed for the iterator-error path. However: validation rules that read property values must skip-with-reason on `Inaccessible` properties rather than failing "required field missing." Concretely, the validator iterates the schema; for each rule, if the rule's target property is in `e.Inaccessible`, the rule is skipped and the skip is recorded (debug log, not error).

10. **Search index.** Inaccessible entities are indexed by ID + Type only (no property values). They are findable by ID/type but do not match property-value searches. A `:locked` filter (or similar) may be added later — out of scope.

11. **Unit tests** cover header detection (zero-byte file, 8-byte file, 9-byte exact match, header followed by ciphertext, partial header `\0GITCRYP\0`, all-NUL file, UTF-8 BOM, normal markdown, ciphertext that contains a git-conflict-marker substring (seven `<` chars at column 0) — must classify as inaccessible not git-conflict; check ordering: magic-header check FIRST).

12. **Integration tests** cover: (a) loading a project with one encrypted entity + one encrypted relation + cleartext metamodel via `fsstore.New` produces entities with full schema in `Inaccessible`, (b) PATCH against an encrypted entity preserves on-disk content and ignores submitted values, (c) PATCH against a normal entity in a project containing encrypted files succeeds (validator does not abort).

13. **Watcher transition.** When a file transitions encrypted → cleartext (simulated via test, equivalent to user running `git-crypt unlock`), the watcher path picks up the new content and the entity reloads with full `Properties`, empty `Inaccessible`. Tested in fsstore watcher tests.

## Approach

### Detection helper

`internal/store/fsstore/gitcrypt.go` (new):

```go
package fsstore

import "bytes"

var gitCryptMagic = []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}

func isGitCryptEncrypted(b []byte) bool {
    return len(b) >= len(gitCryptMagic) && bytes.Equal(b[:len(gitCryptMagic)], gitCryptMagic)
}
```

Called from `readDataFile()` (the single shared read entry point) — closes the
watcher gap (RR-CQCIR) by construction.

### `entity.Entity` extension

`internal/entity/entity.go`:

```go
type Entity struct {
    ID            string
    Type          string
    Properties    map[string]any
    Content       string
    UpdatedAt     time.Time
    Inaccessible  []InaccessibleField  // empty for normal entities
}

type InaccessibleField struct {
    Name   string
    Reason InaccessibleReason
}

type InaccessibleReason string

const (
    InaccessibleReasonGitCrypt InaccessibleReason = "git-crypt"
    // Future: InaccessibleReasonSOPS, InaccessibleReasonACL
)
```

`entity.Relation` gets the same `Inaccessible` field. (Verify Relation has
properties; if not, the field is still useful for content/reason.)

### fsstore read path

`readDataFile` checks the magic header. If detected, returns a sentinel value
(`bytesIsGitCryptEncrypted` or wraps in a typed result) — but the call sites
(`readEntityFile`, `readRelationFile`, `parseEntityFromPath`) need to know to
construct a "shell" entity rather than calling `parseDocument`. Simplest
implementation: return a typed result from a helper:

```go
type readResult struct {
    Bytes      []byte
    Encrypted  bool  // git-crypt detected
}

func (s *Store) readDataFile(key string) (readResult, error)
```

Callers branch:

```go
res, err := s.readDataFile(key)
if err != nil { return nil, err }
if res.Encrypted {
    return s.buildInaccessibleEntity(entityType, id), nil
}
// normal path
```

`buildInaccessibleEntity` consults the metamodel for the entity type's declared
properties and constructs the shell with `Inaccessible` populated.

### Validator

`internal/validator/validator.go` rule application: for each rule, if
`rule.Property` (the field being validated) is in `e.Inaccessible`, skip the
rule and emit a debug log (not an error). This means cardinality and
required-field rules don't fail the project-wide validation pass when encrypted
entities are present. Cross-entity rules (e.g. relation cardinality involving an
inaccessible endpoint) — count the inaccessible endpoint as a real node; the
relation exists, just one side is locked.

### Data-entry API

Wire format: extend `APIEntity` and `APIRelation` with an optional
`inaccessible: [{name, reason}]` field. `properties` field omits any property
that is inaccessible (consistent with the on-`Entity` invariant). The SPA reads
both arrays and renders accordingly.

PATCH handler in `internal/dataentry/handlers_api.go`:

1. Re-read file via fsstore, get fresh on-disk Entity.
2. Build the merged entity: copy on-disk Properties, then for each `(name, value)` in client request, set `merged.Properties[name] = value` **only if `name` not in `on_disk.Inaccessible`**. Else log + drop.
3. Write merged entity. Validation runs on merged state, naturally skipping inaccessible properties (validator change above).

### Data-entry SPA

- `frontend/src/api/entities.ts` — extend types to include `inaccessible?: { name: string; reason: string }[]`.
- `frontend/src/components/lists/EntityList.vue` — for each visible column, if column name in entity.inaccessible, render 🔒. Otherwise render normally.
- `frontend/src/components/entity/EntityDetail.vue` — same logic per form field. Locked fields render as a disabled input with placeholder text "🔒 git-crypt encrypted (run `git-crypt unlock`)" and a help icon.
- `frontend/src/components/forms/DynamicForm.vue` (likely needs a touch) — never submit values for locked fields.

Help affordance: link to the existing in-app help modal (FEAT-8cwr) with a new
help topic `git-crypt-encrypted`. If wiring the modal entry is too much scope,
fall back to an external link to git-crypt's README. **Decide and pin in
implementation, not in plan.**

### Files to modify

**Storage / detection:**
- `internal/store/fsstore/gitcrypt.go` (new)
- `internal/store/fsstore/gitcrypt_test.go` (new)
- `internal/store/fsstore/markdown.go` — `readDataFile` returns the `readResult`; `readEntityFile`/`readRelationFile` branch.
- `internal/store/fsstore/watcher.go` — `parseEntityFromPath` uses the same `readDataFile` so watcher coverage is automatic.
- `internal/store/fsstore/entity.go` — implement `buildInaccessibleEntity`.
- `internal/store/fsstore/relation.go` — `buildInaccessibleRelation`.

**Domain types:**
- `internal/entity/entity.go` — add `Inaccessible` field on `Entity` and `Relation` plus `InaccessibleField` + `InaccessibleReason` enum.
- `internal/entity/entity_test.go` — round-trip tests.

**Metamodel schema lookup helper:**
- A small helper inside `fsstore` that, given an entity type, returns the list of declared property names from the metamodel snapshot the store holds.

**Validator:**
- `internal/validator/validator.go` — skip rules whose target property is in `e.Inaccessible`, log skip at debug level. Tests in same package.

**Data-entry server:**
- `internal/dataentry/handlers_api.go` — PATCH: re-read, merge with skip-locked, re-detect on save. Wire `inaccessible` field into APIEntity/APIRelation responses.
- `internal/dataentry/api_types.go` (or wherever response types live) — extend response shape.
- `internal/dataentry/handlers_api_test.go` — coverage for read + patch + concurrent-encrypt scenarios.

**Data-entry SPA:**
- `frontend/src/api/entities.ts` — types
- `frontend/src/components/lists/EntityList.vue` — locked-cell rendering
- `frontend/src/components/entity/EntityDetail.vue` — locked-field rendering
- `frontend/src/components/forms/DynamicForm.vue` — never submit locked fields
- `frontend/src/components/common/LockedFieldIndicator.vue` (new, shared, includes help affordance)

**E2E:**
- `e2e/tests/fixtures.ts` — helper to drop a fake-git-crypt file (binary fixture, not `printf`)
- `e2e/tests/git-crypt.spec.ts` (new) — list + detail rendering, save-path no-op for locked fields

**Test fixtures:**
- A binary fixture file containing the 10-byte git-crypt magic followed by random bytes, committed at `internal/store/fsstore/testdata/encrypted.bin` (or similar). Used by all tests rather than constructing bytes inline.

### Alternatives reconsidered

- **Sibling iterators (`ListInaccessibleEntities`)** — rejected. Adds interface bloat for non-fsstore backends and forces double-iteration in consumers. The save-path scenario (RR-8LLD0) is decisive: write paths need redaction info to travel with the entity, which only works if it's on the entity itself.
- **Typed error (`*EncryptedError`)** — rejected. Same save-path issue. Also makes the "partial encryption is normal" case painful: every iterator consumer would need explicit error handling.
- **`LoadResult` wrapper** — rejected. Solves read paths but breaks down on save (PATCH receives a partial entity from client; server can't tell "user cleared this field" from "user never saw this field" without consulting redaction context, which has to be re-fetched). Putting it on `Entity` makes redaction travel naturally.
- **FS-layer detection at `storage.RootedFS.ReadFile`** — rejected for this ticket. Under partial encryption, metamodel.yaml is normally cleartext, so the metamodel-coverage benefit is small. Out of scope; revisit if SOPS work changes the analysis.

## Test plan

### Unit tests (header detection)

`internal/store/fsstore/gitcrypt_test.go`, table-driven:

| Case | Bytes | Expected |
|---|---|---|
| exact 9 bytes magic | `\x00GITCRYPT\x00` | true |
| magic + ciphertext | `\x00GITCRYPT\x00...random bytes...` | true |
| 8 bytes (too short) | `\x00GITCRYP\x00` | false |
| 9 bytes near-miss | `\x00GITCRYPS\x00` | false |
| zero-byte file | `` | false |
| all-NUL file | `\x00\x00\x00...` | false |
| UTF-8 BOM | `\xEF\xBB\xBF# Title` | false |
| normal frontmatter | `---\ntype: feature\n---\n` | false |
| ciphertext with conflict-marker substring | magic + bytes containing seven `<` chars | true (regression: ordering — magic check FIRST) |

### Integration tests (fsstore)

- Create a MemFS project: cleartext metamodel, one cleartext entity, one entity file containing magic header + random bytes, one relation file containing magic + random bytes. Open store. Assert:
  - `GetEntity(encrypted)` returns Entity with full schema in `Inaccessible`, no error.
  - `ListEntities` yields all three entities (cleartext + 1 encrypted) with no error in iterator.
  - `GetRelation(encrypted)` returns Relation with `Inaccessible` set, no error.
  - Search index built from this store contains the encrypted entity findable by ID.
- Watcher transition: write a magic-prefixed file, observe entity loads with `Inaccessible`. Overwrite with cleartext markdown, observe entity reloads with full `Properties` and empty `Inaccessible`.

### Validator tests

- Project with one encrypted entity that would normally fail "required title field" — validator passes (rule skipped, debug logged).
- Project with one encrypted entity in middle of a `requires` cardinality chain — cardinality rule still applies; encrypted endpoint counts as a valid node.

### Data-entry handler tests (`internal/dataentry/handlers_api_test.go`)

- GET list: response contains encrypted entity with `inaccessible` array populated, ID + type set, no `properties` for locked fields.
- GET detail: same.
- PATCH against encrypted entity: client posts `{title: "evil"}`, server re-reads, sees title is in Inaccessible, drops the value, writes nothing-changed; assert on-disk file is byte-identical to before.
- PATCH against cleartext entity in project with encrypted siblings: succeeds normally (validator doesn't abort).
- PATCH against entity that became encrypted between SPA load and submit: server re-detects, returns clear error.

### Playwright E2E

- Fixture project with binary git-crypt fixture entity. Navigate list view → assert lock icons in expected cells. Navigate detail → assert form fields show locked indicator. Try to submit form → assert no PATCH for locked fields. Manual reload after fixture is replaced with cleartext file → assert UI updates.

### Manual verification

- Run `just dev` against a real project. Use `python3 -c "import sys; sys.stdout.buffer.write(b'\\x00GITCRYPT\\x00' + b'random bytes')" > entities/feature/FEAT-X.md`. Reload, observe:
  - List view shows FEAT-X with locked cells.
  - Detail page shows locked form fields with help text.
  - Edit attempt for locked fields submits no value; on-disk file unchanged.
- Replace file with cleartext markdown. Observe UI auto-refresh (file watcher) → entity now fully editable.
- Run `rela list` from CLI in same project — entity appears in output with `<inaccessible>` placeholders or similar; no crash.

## Risk assessment

| Risk | Mitigation |
|---|---|
| Adding `Inaccessible` to `entity.Entity` breaks unmarshalling/serialization in unexpected places. | Field is optional (`omitempty`); existing consumers that don't know about it ignore. Audit JSON/YAML round-trip tests in the markdown package. |
| Schema lookup at load time creates a coupling: fsstore must know the metamodel. | fsstore already holds a metamodel reference (per `internal/store/fsstore/index.go` and propcache). No new dependency. |
| Save-path bug: client posts an inaccessible field's value and server writes it. | Server-side enforcement is the source of truth. Test: explicit PATCH-with-locked-field test asserts byte-identical on-disk after operation. |
| Validator skip-on-inaccessible silently passes broken entities. | Skip is logged at debug level + skip count surfaced in validation summary. User running `analyze_validations` sees "5 rules skipped (encrypted)". |
| Watcher transition encrypted → cleartext doesn't reload the entity. | Tested explicitly; the existing watcher already handles file modification events generically. |
| Magic-header false positive on legitimate markdown. | Header is two NUL bytes around `GITCRYPT`. Cannot occur in valid UTF-8 markdown. |
| Data-entry SPA submits a locked field's value because it kept it in form state. | DynamicForm filters out locked field names before serializing. Server is the second-line defense. |
| Frontend rendering bug shows 🔒 instead of value for non-locked field. | E2E tests cover both cleartext and encrypted entities in the same fixture. |

## Open questions resolved

- Detection layer — fsstore via `readDataFile` (single insertion; covers entity, relation, watcher). ✓
- Generality — git-crypt-only for v1; design extensible to SOPS / ACL via `InaccessibleReason` enum. ✓
- Where redaction lives — on `entity.Entity` itself, as `Inaccessible []InaccessibleField`. Travels with the entity through read and write paths. ✓
- Sibling iterator vs typed error — neither. Field on `Entity` makes both redundant. ✓
- Naming — `Inaccessible` over `Redacted` (reason-neutral, scales to ACL/SOPS). ✓
- Help link destination — in-app help modal preferred, external link as fallback. Decide in implementation. ✓
- mtime/etag concurrency check — out of scope; PATCH re-detects encrypted state on save. ✓

## Effort

Still `m`. Distribution:
- fsstore detection + tests: small
- `entity.Entity` + `Relation` extension: small
- Validator skip-on-inaccessible: small
- Data-entry handler (read + PATCH merge): small-medium
- SPA locked-field rendering + form filtering: medium
- E2E: small

The pivot from sibling iterator → field-on-entity actually reduces code volume
(no new interface methods, no merge logic in handlers). The trade is touching
`entity.Entity`, which is intentional for the save-path correctness reasons.
