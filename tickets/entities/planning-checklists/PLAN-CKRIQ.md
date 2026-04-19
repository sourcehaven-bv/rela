---
id: PLAN-CKRIQ
type: planning-checklist
title: 'Planning: Metamodel parsing of encrypted: declarations + groups config (slice 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Slice 2 of FEAT-JPJ2C. Teach the metamodel layer how to parse `encrypted:
<group>` on properties and entity bodies, and introduce a top-level
`groups.yaml` config that maps group names to recipient identities.

This slice ships **metadata only** — no crypto, no file read/write, no
integration. Slice 3 consumes the parsed data.

**In scope:**

- `PropertyDef.Encrypted string` (new YAML field)
- `EntityDef.EncryptedBody string` (new YAML field)
- `internal/metamodel/groups.go` with `Groups`, `LoadGroups`, `Recipients`, `Contains`
- Typed sentinels: `ErrGroupsNotFound`, `ErrUnknownGroup`, `ErrDuplicateIdentity`
- Validation: every `encrypted:` reference resolves to a known group
- Tests ≥ 90% coverage of new code

**Out of scope (explicitly):**

- Actual encryption/decryption on read/write — slice 3
- `fsstore` integration, `!enc` YAML tags — slice 3
- Identity ↔ `.pub` cross-validation — belongs at wiring site, not in metamodel
- CLI commands — slice 5
- Key rotation, key versions — slice 4

**Acceptance Criteria:** see ticket (10 criteria, each mapped to a test in §Test
Plan).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

Patterns in codebase to mirror:

- `internal/metamodel/types.go` — `PropertyDef` already has orthogonal string fields (`Format`, `Default`, etc.). Adding `Encrypted` follows the same pattern — no schema refactor needed.
- `internal/ai/config.go` / `internal/ai/loader.go` — the "self-contained loader, file in project root, typed sentinels, missing-file is not an error" pattern we'll mirror for `Groups`.
- `internal/migration/runner.go` — shows how a separate YAML file in the project root is loaded and validated alongside `metamodel.yaml`.
- `internal/encryption/keyring.go` from slice 1 — loads recipient pubkeys keyed by filename; `Groups` provides the list of *which* of those identities belong to a group. The two are composable but intentionally decoupled: encryption doesn't know about groups, metamodel doesn't know about keys.

External tools reviewed:

- **SOPS groups**: YAML groups config that maps group → list of recipient fingerprints. Our format is simpler (identities, not fingerprints) because the identities are just filenames in `keys/`.
- **age recipients**: flat list, no grouping. Doesn't apply.

**Rela concepts reviewed:**

- `encryption` concept doc (created in slice 1) — documents the file-format shape. This slice implements the metamodel side of that contract.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

(Incorporates S1–S5 and M1–M4, N1–N3 from the go-architect design review.)

1. **Extend `PropertyDef`**: add `Encrypted string \`yaml:"encrypted,omitempty"\``with a doc comment explaining this is the *metamodel attribute* — unrelated to the on-disk`!enc` YAML tag that slice 3 will introduce (N2).

2. **Extend `EntityDef`**: add `EncryptedBody string \`yaml:"encrypted_body,omitempty"\``. Doc comment commits to **exactly one group per body** (S5).

3. **Helpers on `EntityDef`** — ship both shapes, each targets a different caller (S4):
   - `EncryptedProperties() map[string]string` — `{propName: groupName}`. Validation path ("is every reference resolvable?").
   - `PropertiesByGroup() map[string][]string` — `{groupName: [propName...]}`. Read/write path ("for group G, which properties go into its envelope?").
   - `BodyGroup() (string, bool)` — single group, `(_, false)` when cleartext.

4. **New file `internal/metamodel/groups.go`**:
   - `type Groups struct { groups map[string][]string }` — unexported map.
   - `LoadGroups(projectRoot string, fs storage.FS) (*Groups, error)` — takes storage.FS to match existing `metamodel.Load` pattern (S1). Uses `yaml.NewDecoder(bytes.NewReader(data)).KnownFields(true)` — strict mode on a greenfield file catches typos like `enginerring:` (M2). Missing file returns a sentinel-wrapped error.
   - `(g *Groups) Recipients(group string) ([]string, bool)` — returns the slice directly with a `// Do not mutate.` godoc comment; no defensive copy (M3).
   - `(g *Groups) Contains(group string) bool`.
   - No `All()` until a caller exists (M4).

5. **Typed error in `internal/metamodel/groups_errors.go`** (M1) — mirrors `internal/ai/errors.go`'s shape:
   ```go
   type GroupErrorKind string
   const (
       GroupErrorNotFound  GroupErrorKind = "not_found"
       GroupErrorUnknown   GroupErrorKind = "unknown"
       GroupErrorDuplicate GroupErrorKind = "duplicate_identity"
   )
   type GroupError struct {
       Kind     GroupErrorKind
       Group    string  // "engineering" — empty for file-level NotFound
       Path     string  // "entities.ticket.properties.description" — for Unknown
       Identity string  // for Duplicate
   }
   func (e *GroupError) Error() string { ... }
   func (e *GroupError) Is(target error) bool { ... }

   var (
       ErrGroupsNotFound    = &GroupError{Kind: GroupErrorNotFound}
       ErrUnknownGroup      = &GroupError{Kind: GroupErrorUnknown}
       ErrDuplicateIdentity = &GroupError{Kind: GroupErrorDuplicate}
   )
   ```
Callers can still use `errors.Is(err, ErrUnknownGroup)`; slice 3 can
`errors.As(err, &ge)` for structured context.

6. **Validation as a method in `validation.go`** (S3):
   - `func (m *Metamodel) ValidateEncryption(g *Groups) error` — iterates entity defs, collects referenced groups via `EncryptedProperties()` and `BodyGroup()`, checks each via `g.Contains()`.
   - Short-circuits to nil if no encryption is declared anywhere (missing-groups is fine iff no references).
   - If encryption declared + `g == nil` → returns `&GroupError{Kind: GroupErrorNotFound}`.
   - If unknown group → returns `&GroupError{Kind: GroupErrorUnknown, Group: X, Path: "entities.<type>.properties.<name>"}`.

7. **Do NOT auto-wire groups into `metamodel.Load`** (S2). Keep `Load` pure. Add a thin helper for the common "I want both":
   ```go
   func LoadWithGroups(path string, fs storage.FS) (*Metamodel, *Groups, []string, error)
   ```
that calls `Load` + `LoadGroups` + `ValidateEncryption` and returns all three.
Call sites (cli, mcp, dataentry) pick whichever entry point fits their
error-surfacing policy.

**Alternatives considered:**

- **Groups inline in `metamodel.yaml`**: rejected. Groups change with team membership; metamodel changes with schema. Separating them lets one file move without the other.
- **Groups in `.rela/groups.yaml`**: rejected. `.rela/` is for per-user / gitignored-adjacent state (cache, AI config). Groups are public team-membership facts that belong in the repo.
- **Per-entity-type groups**: rejected as YAGNI. A group applies project-wide; slice 2 doesn't need per-type scoping.
- **Identity ↔ pubkey cross-check in metamodel**: rejected. Would force `internal/metamodel/` to import `internal/encryption/`, violating the layering. That check belongs at the wiring site when Groups + Keyring are both in scope.
- **`Encrypted bool` instead of `Encrypted string`**: rejected. Boolean loses the group attribution, and "which group" is the information the integration layer actually needs.

**Dependencies:**

- `gopkg.in/yaml.v3` — already in go.mod (used by existing metamodel loader)
- `os`, `path/filepath`, `errors`, `fmt` — stdlib
- **No** `internal/encryption` import (architecture constraint)

**Files to modify:**

- `internal/metamodel/types.go` — 2 field additions
- `internal/metamodel/entity_def.go` — 2 helper methods
- `internal/metamodel/groups.go` — new
- `internal/metamodel/groups_test.go` — new
- `internal/metamodel/validation.go` — `ValidateEncryption`
- `internal/metamodel/validation_test.go` — new tests
- `internal/metamodel/loader.go` — call into `LoadGroups` + `ValidateEncryption`
- `internal/metamodel/errors.go` — 3 sentinels (or new file)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | Invalid handling |
|---|---|---|
| `metamodel.yaml` `encrypted:` value | String; non-empty means "group name"; arbitrary characters allowed (matches file-naming) | Passed through; validated against groups config |
| `groups.yaml` file | YAML with `groups: map[string][]string` shape | Parse errors → wrapped `fmt.Errorf` with line info |
| Group names in `groups.yaml` | Map keys from YAML | No further validation (schema allows any string) |
| Identities in `groups.yaml` | List elements | Must be non-empty; duplicates within a single group rejected with `ErrDuplicateIdentity` |

Groups file is NOT secret material — it's a public team roster. No redaction
needed in error messages. Group names and identities can safely appear in
validation errors.

**Security-Sensitive Operations:**

- **File access**: `os.ReadFile("<projectRoot>/groups.yaml")`. Standard path handling. No symlink following is special-cased (rely on stdlib behavior).
- **No crypto**: this slice doesn't touch keys or ciphertext.
- **Attack surface**: a malicious `groups.yaml` could claim that an attacker's identity is in the `engineering` group. But since identities in `groups.yaml` must also exist as `keys/<id>.pub` files on disk (cross-checked at the wiring site, not here), and the repo itself is the trust boundary for `keys/`, a groups-file attack reduces to a repo-content attack.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (acceptance criterion → test):**

1. Property parse — `TestPropertyDef_EncryptedField`: YAML snippet with `encrypted: engineering` on a property; assert round-trip.
2. Body parse — `TestEntityDef_EncryptedBody`: YAML with `encrypted_body: exec`; assert `BodyGroup()` returns `("exec", true)`.
3. Groups loader — `TestLoadGroups_Basic`: tempdir fixture with 3 groups; assert `Recipients` ordering + `Contains`.
4. Missing + no encryption — `TestValidateEncryption_NoEncryptionNoGroups`: metamodel without any `encrypted:`; `g = nil`; no error.
5. Missing + encryption declared — `TestValidateEncryption_NoGroupsButEncrypted`: metamodel with `encrypted: engineering`; `g = nil`; expect `ErrGroupsNotFound`.
6. Unknown group — `TestValidateEncryption_UnknownGroup`: `encrypted: ghost` but groups defines only `engineering`; expect `ErrUnknownGroup` and path `entities.<type>.properties.description`.
7. Duplicate identity — `TestLoadGroups_DuplicateIdentity`: groups.yaml with `engineering: [alice, bob, alice]`; expect `ErrDuplicateIdentity` naming "alice".
8. Accessor happy path — `TestGroups_RecipientsOrdered`: declared `engineering: [charlie, alice, bob]`; assert returned slice matches declaration order (not sorted).
9. Arch-lint — `go-arch-lint check` passes (no new `internal/encryption` import from metamodel).
10. Coverage — `go test -cover ./internal/metamodel/` ≥ 90% on the new code paths.

**Edge Cases:**

- Empty `groups.yaml` (zero groups) — loads cleanly; any `encrypted:` reference fails `ErrUnknownGroup`.
- Empty group `engineering: []` — loads cleanly; validation emits a warning (low severity) or leaves it to the wiring site. **Decision**: leave it permissive in slice 2; wiring site in slice 3 will surface an empty recipient list when it tries to wrap.
- Group name with whitespace or unicode — accepted; case-sensitive throughout.
- Identity with spaces or punctuation — accepted; case-sensitive (matches filename semantics).
- Huge groups (1000+ identities) — `LoadGroups` handles via `make([]string, 0, n)`; no aliasing between parsed slice and returned slice.
- `groups.yaml` with extra top-level keys — YAML decoder ignores unknown keys by default; no strict-mode enabled for this file to keep it forgiving.
- Symlink at `groups.yaml` — followed (stdlib default). No special handling.
- Windows-style line endings — YAML parser tolerates.

**Negative Tests:**

- `groups.yaml` with malformed YAML — wrap parse error with filename context.
- `groups.yaml` present but empty file — parses as `{Groups: nil}`; same as "no groups defined".
- `groups.yaml` with non-map root — YAML type mismatch error surfaced.
- `encrypted:` on a property type that doesn't make sense (e.g., `encrypted: x` on a `type: integer` property) — allowed in slice 2, with a planning-time note. Slice 3 will decide if integer encryption is a real use case (probably yes — values get serialised as strings before seal).
- `encrypted: ""` (empty group reference) — treated as "not encrypted", same as absence. Documented.

**Integration test approach:**

End-to-end test in `TestLoadMetamodelWithEncryption` (validation_test.go):

1. Tempdir with `metamodel.yaml` declaring two entity types, one with `encrypted: engineering` on a property.
2. `groups.yaml` with `engineering: [alice, bob]`.
3. Call the existing metamodel loader entrypoint; verify it returns `(m, nil)`.
4. Assert `m.Entities["ticket"].EncryptedProperties() == {"description": "engineering"}`.
5. Repeat without `groups.yaml`: expect loader error wrapping `ErrGroupsNotFound`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **API shape drift with slice 3**: the `Groups` accessor signature (`Recipients`, `Contains`) needs to match what slice 3's wiring needs. Mitigation: slice 3 follows immediately; if we realise in slice 3 we want a different API, it's a small refactor inside the same feature branch rather than a stable public API change.
- **Layering regression**: a tired contributor might import `internal/encryption` from a helper. Mitigation: add `encryption` to the "must not depend on" list for `metamodel` in `.go-arch-lint.yml`.
- **YAML extension point clash**: some metamodel users might already have a property named `encrypted` in their schema (unlikely, but possible). Mitigation: `encrypted:` in property def is a new YAML key, not a property name. The schema change is orthogonal.
- **Groups file format baked in**: the `groups.yaml` shape is ossified once a user adopts it. Mitigation: keep the top-level `groups:` nesting so we have a place to add sibling keys (like `schema_version:`) later.

**Effort**: **m** — ~1 day. Parsing + 3 helpers + validation + tests.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: slice 2 is metamodel plumbing with no user-facing features; docs land with slice 5/6 when CLI + UI ship)
- [x] ~~CLI help text~~ (N/A: no commands change)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns; existing "metamodel YAML gets new field" is already documented by example)
- [x] ~~README~~ (N/A)
- [x] ~~Docs-checklist~~ (N/A: internal change, no user-facing surface; docs land in slices 5/6)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Reviewed with `go-architect`. No critical findings. Significant + minor findings
all addressed in the approach above (no open review-response entities; fixed in
plan):

- **S1**: `LoadGroups(projectRoot, fs)` takes `storage.FS` (not `os.ReadFile`
directly) — matches existing `metamodel.Load` contract and keeps tests
in-memory.
- **S2**: Groups are NOT auto-loaded by `metamodel.Load`. Added explicit
`LoadWithGroups` helper for the common case.
- **S3**: `ValidateEncryption` is a method on `*Metamodel`, not a free function.
- **S4**: Ship both `EncryptedProperties()` (validation shape) and
`PropertiesByGroup()` (read/write shape) on `EntityDef`.
- **S5**: `BodyGroup()` returns a single group; documented in the field
comment.
- **M1**: Replaced three parallel sentinels with a `*GroupError{Kind, Group,
Path, Identity}` typed error + sentinel matchers for `errors.Is`.
- **M2**: `groups.yaml` parsed with `yaml.NewDecoder.KnownFields(true)` — strict
mode catches typos.
- **M3**: `Recipients()` returns the slice directly with a `// Do not mutate.`
godoc (no defensive copy).
- **M4**: Dropped `All()` — add when a caller exists.
- **N1**: Kept `EncryptedBody` / `BodyGroup()` naming asymmetry; documented.
- **N2**: Doc comment on `PropertyDef.Encrypted` explicitly notes it's unrelated
to the slice-3 `!enc` YAML tag.
- **N3**: Doc comment on `type Groups` notes the top-level `groups:` nesting
leaves room for future sibling keys.
