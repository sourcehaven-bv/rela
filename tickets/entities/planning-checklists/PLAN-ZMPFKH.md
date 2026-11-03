---
id: PLAN-ZMPFKH
type: planning-checklist
title: 'Planning: Lua rela.tagged_entities: find entities by version tag'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Store capability `store.VersionTagIndex.TaggedEntities(ctx, name, type)` on
pgstore and sqlitestore VersionStore, pinned by `storetest` (tag suite).
- `visibility.ScriptReader.TaggedEntities` (gated), `UnrestrictedReader`
(ungated), `DenyReader` (empty).
- Lua binding `rela.tagged_entities(name [, {type=}])` registered beside
`rela.version_by_tag` (read binding, so available on every runtime that reads,
including documents and validation).
- Docs: Lua scripting guide (reference + "baseline and compare" example).

Out of scope (backlog tickets):

- Version tags on relations.
- Graph-wide `as_of` reads for views and exports.
- Reading the tagged snapshot of a DELETED entity from Lua (`version_by_tag`
and `get_version` need a live face today). `tagged_entities` reports the deleted
entity's id, face, type and version; content comes later.

**Acceptance Criteria:**

1. A script lists every entity with a version tagged `name`, with `id`, `face`,
`addr`, `type`, `version`, `deleted`. Test: storetest + sqlite/pg appbuild
integration test.
2. `{type = "x"}` restricts to that type. Test: same.
3. An entity deleted after it was tagged is listed with `deleted = true` for a
holder of `history:read`, and omitted for a non-holder. Test: appbuild
integration with two principals.
4. A live entity the principal cannot read is omitted. Test: same.
5. A renamed entity is listed under its current id, and `version` equals what
`rela.version_by_tag(addr, name).version` returns. Test: storetest lifecycle
cases.
6. One store query per call. Test: statement count via the sqlite/pg backend
does not grow with rows (budget test with `storetest.Counting` around the header
batch; store method is one SQL statement by construction).
7. fs and memory backends raise "version history is not supported". Test:
Lua unit test with a reader without the capability.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small extension of TKT-VO6VG9; approach follows existing
patterns)

**Existing Solutions:**

- `internal/store/pgstore/version.go` `lineageCTE` and
`internal/store/sqlitestore/version.go`: the backward fenced lineage walk that
assigns ordinals. The new query reuses its hop rule.
- `internal/store/storeutil/versiontags.go` `CurrentLifecycle`: a delete newer
than the row on the same id ends the lifecycle. The new forward walk stops at
the same delete.
- `internal/dataentry/historyworld.go` `resolveHistorySubject`: the deleted-face
rule (history:read + face half of the read grant + a readable live sibling when
one exists). Mirrored in visibility.
- `internal/visibility/batch.go` `storedHeaders`/`gateHeaders`: one header
query plus one gate probe per type.
- No library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Store (`internal/store/store.go`): new optional capability

```go
type TaggedEntity struct {
    Ref     entity.Ref // current id+face; for a deleted entity, its last id+face
    Type    string     // live type; for a deleted entity, the tagged version's type
    Version int        // ordinal of the tagged version in the face's lineage
    Deleted bool
}
type VersionTagIndex interface {
    TaggedEntities(ctx, name VersionTagName, entityType string) ([]TaggedEntity, error)
}
```

pgstore and sqlitestore implement it as ONE SQL statement:

- `tagged`: version_tags rows of `name` joined to their entity_versions row.
- `fwd` (recursive): from the tagged row follow the next rename that moved the
(id, face) away, unless a delete of that (id, face) lies between. The last hop
is the head.
- `bwd` (recursive): the lineageCTE hop rule seeded at the tagged row, bounded
above by it; `count(DISTINCT vseq)` is the ordinal, equal to ListVersions'.
- deleted = a delete row on the head after the last hop, or no live row
(covers soft delete).
- type filter and ORDER BY id, face in an outer SELECT.

No index on `version_tags(name)`: the table holds one row per tagged version,
and a scan stays cheap. Adding one would need a migration on both backends
(noted as a follow-up if profiling asks for it).

Visibility (`internal/visibility/history.go`): `ScriptReader.TaggedEntities`:

1. ErrHistoryUnsupported when the tag reader lacks the capability.
2. Store rows. 3. One `storedHeaders` read for all ids, one `gateHeaders` pass
(one ReadableFacesMany per type).
3. Live row: kept when its own face is readable and the stored type matches.
4. Deleted row: kept only when the gate's `HoldsPermission(history:read)`
holds, the face half of the read grant allows the face (`FaceAllowed`), and,
when the id still has live rows, they are of the same type and one is readable.
A gate without `HoldsPermission` fails closed (deleted rows omitted). A denied
world returns nothing. `DeclarativeGate` gains `HoldsPermission(ctx, perm)`.
`UnrestrictedReader` returns every row; `DenyReader` returns none.

Lua (`internal/lua/versiontags.go`): `rela.tagged_entities(name [, opts])`. Bad
name or opts raise; a reader without the capability raises
ErrHistoryUnsupported; other errors raise a generic message and are logged
(historyAnswer pattern). Returns an array of `{id, face, addr, type, version,
deleted}`.

**Alternatives considered:**

- Per-row lineage reads (listLineage per tag): violates the one-query rule.
- Return only rows in the current lifecycle (like `version_by_tag`): drops
deleted entities, the point of the ticket.
- Gate deleted rows on `history:read` only: weaker than the HTTP history rule
(face grant + live sibling); rejected so Lua and HTTP agree.

**Files to modify:**

- internal/store/store.go
- internal/store/pgstore/versiontags.go (+ query)
- internal/store/sqlitestore/versiontags.go (+ query)
- internal/store/storetest/versiontags.go
- internal/visibility/history.go, adapters.go
- internal/lua/versiontags.go, versiontags_test.go
- internal/appbuild/luataggedentities_db_test.go (new, sqlite || postgres)
- docs-project Lua scripting guide entity

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Tag name from the script: `store.ParseVersionTagName` (allowlist grammar),
raise on invalid. Bound as a SQL parameter.
- `type` option: string, bound as a SQL parameter; unknown type yields none.

**Security-Sensitive Operations:**

- Existence of deleted entities: gated on `history:read` plus face grant and
live-sibling readability, as the HTTP history endpoint does.
- Existence of hidden live entities: gated by the row gate per face; a hidden
row is omitted, not reported.
- Rows carry no property values, so no field redaction is needed.
- Store errors are logged, not shown to the script (SQL text).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- storetest `TaggedEntities` cases (run on sqlite and postgres conformance):
plain, type filter, rename follows (version equals VersionByTag), delete
(deleted=true), delete+recreate (old deleted row, new untagged), rename onto
deleted id (old deleted, new live), per-face, multiple versions (ordinal 2),
soft delete reads deleted, unknown name empty.
- visibility unit tests with a fake gate: live hidden omitted, deleted
without permission omitted, deleted with permission but face not granted
omitted, deleted with unreadable live sibling omitted, denied world empty.
- Lua unit tests: shape of rows, bad name raises, bad opts raise, reader
without capability raises.
- appbuild integration (sqlite || postgres): alice (admin + history:read)
sees live + deleted; bob (reads one type) sees only that type's live rows;
deleted rows omitted for bob; baseline-and-compare script from the docs runs.

**Edge Cases:**

- No tags of that name: empty table.
- Entity tagged on two faces: two rows.
- Same id deleted and recreated: deleted row plus (if tagged) live row.
- Type converted after tagging: live type is reported.

**Negative Tests:**

- Invalid tag name raises with the grammar message.
- `{type = 5}` raises.
- fs/memory backend raises "version history is not supported".

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Ordinal mismatch with ListVersions across renames: pinned by storetest
comparing against VersionByTag on both backends.
- Recursive CTE dialect differences: each backend has its own SQL; both run
the same conformance suite.
- Rule drift from the HTTP deleted-history gate: the visibility gate mirrors
`resolveHistorySubject`; documented in both places.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Lua scripting guide (docs-project source): `rela.tagged_entities`
reference and a "baseline and compare" example.
- [x] docs/acl-security.md history section: one line that the Lua binding
follows the same rule.

## Design Review

- [x] Run `/design-review` before starting implementation (5 findings: RR-0364LX, RR-LCH63A, RR-74R3I1, RR-J64DJR, RR-P3FDUY)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** pending
