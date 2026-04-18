---
id: TKT-OGLXI
type: ticket
title: 'Metamodel parsing of encrypted: declarations + groups config (slice 2)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Slice 2 of FEAT-JPJ2C: teach the metamodel layer to parse `encrypted: <group>`
declarations on properties and entity bodies, and introduce a top-level
`groups.yaml` config that maps group names to recipient identities.

This slice ships **metadata only** — no file encryption or decryption happens
here. Consumers (slice 3's `fsstore` integration) will query these structures to
decide what to encrypt and for whom.

## Scope

### In scope

- **`encrypted:` property attribute**: extend `PropertyDef` with an `Encrypted string` field. Empty means cleartext (default). Non-empty is the group name whose recipients must be able to decrypt this property.
- **Body encryption**: extend `EntityDef` with an `EncryptedBody string` field. Same semantics: empty → cleartext body, group name → body encrypted for that group.
- **`groups.yaml` loader**: a new `internal/metamodel/groups.go` loads `<projectRoot>/groups.yaml` into a typed `Groups` struct.
- **Consistency validation**: metamodel load-time check that every group referenced by `encrypted:` exists in `groups.yaml`. Missing `groups.yaml` is tolerated iff no `encrypted:` declarations exist.
- **Recipient-list API**: `Groups.Recipients(group string) ([]string, bool)` — returns the ordered identity list.
- **Metamodel-query API**: `EntityDef.EncryptedProperties() map[string]string` returning `{propertyName: groupName}`. `EntityDef.BodyGroup() (string, bool)`.
- **Typed errors**: `ErrGroupsNotFound`, `ErrUnknownGroup`, `ErrDuplicateIdentity`.
- **Tests**: metamodel with no encryption (baseline), property-only, body-only, mixed, references to missing group, `groups.yaml` parse errors, duplicate identities.

### Out of scope (deferred to slice 3+)

- Actually reading/writing `!enc` YAML tags in entity files
- `fsstore` integration, key-version tracking
- CLI commands for group management
- The groups file's relationship with the `keys/` directory (no cross-validation that every identity has a `.pub` file — defer to the wiring site since `internal/metamodel/` shouldn't import `internal/encryption/`)

## File-format decision (documented for slice 3)

The `groups.yaml` file lives at `<projectRoot>/groups.yaml` (alongside the
`keys/` directory), not inside `metamodel.yaml` and not inside `.rela/`. Reason:
groups are public team-membership facts (same as `keys/*.pub`) that belong in
the repo; they change with the team, not with the schema; they should be
committed (not gitignored).

## Design Sketch

```go
// internal/metamodel/types.go additions
type PropertyDef struct {
    // ... existing fields ...
    // Encrypted is the group name whose recipients can decrypt this
    // property. Empty means cleartext.
    Encrypted string `yaml:"encrypted,omitempty"`
}

type EntityDef struct {
    // ... existing fields ...
    // EncryptedBody is the group name whose recipients can decrypt the
    // markdown body. Empty means cleartext body.
    EncryptedBody string `yaml:"encrypted_body,omitempty"`
}

func (e *EntityDef) EncryptedProperties() map[string]string
func (e *EntityDef) BodyGroup() (string, bool)

// internal/metamodel/groups.go (new file)
type Groups struct {
    Groups map[string][]string `yaml:"groups"`
}

func LoadGroups(projectRoot string) (*Groups, error)
func (g *Groups) Recipients(group string) ([]string, bool)
func (g *Groups) Contains(group string) bool

var (
    ErrGroupsNotFound    = errors.New("metamodel: groups.yaml not found")
    ErrUnknownGroup      = errors.New("metamodel: unknown group")
    ErrDuplicateIdentity = errors.New("metamodel: duplicate identity in group")
)
```

## Files

- `internal/metamodel/types.go` — add `Encrypted` / `EncryptedBody` fields
- `internal/metamodel/entity_def.go` — add `EncryptedProperties()` / `BodyGroup()` helpers
- `internal/metamodel/groups.go` — new: `Groups` type + loader
- `internal/metamodel/groups_test.go` — new: loader tests
- `internal/metamodel/validation.go` — add consistency check
- `internal/metamodel/validation_test.go` — add tests

## Acceptance Criteria

1. `metamodel.yaml` with `encrypted: engineering` on a property parses; `EntityDef.EncryptedProperties()` returns that mapping.
2. Entity def with `encrypted_body: exec` parses; `BodyGroup()` returns `("exec", true)`.
3. `LoadGroups` reads `<projectRoot>/groups.yaml` and returns `*Groups` with correct recipient lists.
4. Missing `groups.yaml` + no `encrypted:` declarations → no error; metamodel loads cleanly.
5. Missing `groups.yaml` + any `encrypted:` declaration → validation error referencing `ErrGroupsNotFound`.
6. `encrypted: unknown-group` when `groups.yaml` exists → validation error referencing `ErrUnknownGroup` and the property path.
7. `groups.yaml` with duplicate identity in the same group → `ErrDuplicateIdentity` with the identity name.
8. `Groups.Recipients("engineering")` returns the ordered list as declared; unknown group returns `(_, false)`.
9. No import from `internal/encryption/` — the metamodel layer shouldn't know about keys or crypto.
10. Test coverage ≥ 90% for new code.

## Dependencies

- TKT-16RY1 (slice 1) — needs `internal/encryption` to exist, though this slice doesn't import from it.

## Risk Assessment

- **Low**: schema parsing only. No runtime crypto. Main risk is API shape decisions that bite slice 3 — which is why slice 3 follows immediately.
- Cross-package coupling risk: `internal/metamodel/` must NOT import `internal/encryption/`. Identity-to-`.pub` cross-validation belongs at the wiring site.

## Effort

m — ~1 day. Most of it is the groups loader + validation + tests.
