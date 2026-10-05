---
id: PLAN-6IMLL7
type: planning-checklist
title: 'Planning: Store API takes entity.Ref; zero-value EntityQuery is invalid'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (design `.ignored/stage2-design.md`, rulings in section 10, review
amendments in section 11):

- `GetEntity(ctx, entity.Ref)`; `GetEntityState`, `GetEntityAt`,
`StateGetter` and `PropertyValues` removed.
- Family operations named as such: `DeleteFace`, `DeleteFamily`,
`RenameFamily`, `AttachFamilyFile` and the other `...Family...` attachment
methods.
- `entity.RelationKey` on `Get/Create/Update/DeleteRelation`; the `*State`
relation variants and `RelationData.FromFace` removed (ruling D2). The new
accessor is `Relation.Identity()`; `Relation.Key() string` stays (A1).
- `store.FaceSelection` required on `EntityQuery` and `GraphQuery` (D3); the
zero value is `ErrInvalidQuery` on all four backends.
- `HistoryReader` on `Ref`, `VersionMeta.Face` (TKT-7R0ABK, building on
Stage 1 PR 6).
- pg migration 0018 and sqlite rung 8 read indexes; `HighestID` range scans.
- `worlds.Compiled.Default()` seam; address readers renamed `GetAddress`.
- BUG-J3PBFN (write paths) and BUG-95W7MV (analysis and trace).
- Guards: zero-face allowlist deleted; `bareref`, `DefaultWorld()` and
`EntityQuery` literal guards added; `directread` retargeted.

Out of scope: derived list and query indexes (D6, Stage 3); observers, the sync
manifest's default-world protocol and renaming `FaceIn` (D8); the default world
for faced types (TKT-7IZHP0, Stage 3); widening ACL principal lookup and
traversal to faced types (A5, Stage 3).

**Acceptance Criteria:**

1. `storetest` passes on fs, mem, pg and sqlite. Test: `go test ./...` and
`just test-postgres`.
2. No store read has an implicit face: `GetEntity` takes a `Ref`, and a zero
`FaceSelection` is `ErrInvalidQuery`. Test: storetest selection cases for
`ListEntities`, `ListEntitiesPage`, `CountEntities`, `ListEntityHeaders` (native
and fallback) and `GraphQuery`.
3. `Ref{ID, ""}` on a faced family, the zero Ref, `Ref{ID: "X@draft"}` and
malformed faces (`../x`, `a@b`, NUL) are `ErrNotFound`. Test: `RunAddressTests`.
4. `AttachFamilyFile` succeeds when only named faces exist and fails with
`ErrNotFound` when none do, on all four backends.
5. Two edges on one triple with different tails are independent under every
relation method. Test: storetest relation cases.
6. `ListVersions(Ref)` of one face never includes another face's versions,
across a rename and an id reuse; `VersionMeta.Face` is set.
7. Read indexes serve faced rows. Test: the EXPLAIN table in design 4.4 on
pg and sqlite.
8. BUG-J3PBFN: CLI, MCP, Lua delete, automation and import work on a faced
fixture with `ID` and `ID@face`. Test: design 6.3 plus the pg
`DeleteEntityState` race test (A2).
9. BUG-95W7MV: every analyze check and trace shows faced types, and face
lists contain only faces the principal may read (A4). Test: mixed fixture on
CLI, MCP and data-entry analyze.
10. The zero-face allowlist is gone and the new guards pass with
shrink-only allowlists. Test: `internal/archguard`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal store API refactor; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the shape follows RES-Y6JA37 option C and the repo's own Stage 1 patterns)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Y6JA37 (option C, Stage 2), DEC-NPZICR.

**Existing Solutions:**

- `entity.Ref` and `visibility.Resolver` from Stage 1 (TKT-2528AB).
- `directReadAllowlist` in `internal/archguard` is the model for the new
shrink-only guards.
- `lockFamily` in `pgstore/entity.go` already serializes `DeleteEntity`;
`DeleteFace` reuses it (A2).
- `ordersql_explain_test.go` and `graphquery_explain_test.go` are the
precedent for the plan tests.
- `storetest.RunGraphDifferential` against `graphquerynaive` checks the SQL
builders.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Eight PRs into `faces-intrinsic` (design section 7), each green on its own:

1. Indexes: pg `0018_face_read_indexes.sql`, sqlite rung 8 in
`internal/sqlitedb`, `HighestID` range rewrites, EXPLAIN tests. May start now.
2. Seams: `entity.RelationKey` with `Relation.Identity()`,
`worlds.Compiled.Default()` wired to CLI, tracer, scheduler and mail, address
readers renamed `GetAddress`. Starts after Stage 1 PR 6 merges.
3. BUG-J3PBFN on the current API, plus the `DeleteEntityState` lock fix.
4. BUG-95W7MV on the current API, with per-principal face lists.
5. Remaining CLI and service reads; `entityReader.getEntity` deleted.
6. Entity flip: `GetEntity(Ref)`, family renames, history on `Ref`, guards.
7. Relation flip: `RelationKey` on every relation method.
8. Query flip: `FaceSelection` on `EntityQuery` and `GraphQuery`, literal
classification (A5), `face = ''` audit, identity anchor and EndpointMatch (D5,
A6), with a `rela-security-reviewer` pass.

PRs 1 and 2 run in parallel; 3, 4 and 5 run in parallel after 2; 6, 7 and 8
merge one at a time in the order 8, 6, 7.

Alternatives rejected: flipping the API first (a codemod would spell the
zero-face bug explicitly, design section 7); keeping `GraphQuery.World` (D3);
changing `GetRelation` alone (D2); deleting the guard outright (design section
1).

**Files to modify:**

- `internal/entity/ref.go`, new `RelationKey` in `internal/entity`.
- `internal/store/store.go`, new `internal/store/faceselect.go`,
`internal/store/storeutil`, `internal/store/graphquery.go`.
- `internal/store/{fsstore,memstore,pgstore,sqlitestore}`,
`internal/store/graphquerynaive`, `internal/store/storetest`.
- `internal/store/pgstore/migrations/0018_face_read_indexes.sql`,
`internal/sqlitedb/migrate.go`, `internal/sqlitedb/sqlitedb.go`.
- `internal/worlds`, `internal/visibility`, `internal/acl`,
`internal/entitymanager`, `internal/autocascade`, `internal/importer`,
`internal/tracer`, `internal/analysis`, `internal/dataentry`, `internal/cli`,
`internal/mcp`, `internal/lua`, `internal/appbuild`, `internal/aclmap`,
`internal/docs`, `internal/datamigration`.
- `internal/archguard` (guards).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Wire addresses (`ID@face`) from HTTP, MCP, Lua and CLI: parsed only by the
resolver and the CLI helper; the store does no parsing.
- `Ref` values reaching the store: an invalid id or face (`X@draft`,
`../x`, `a@b`, NUL) is `ErrNotFound`, never a path error (A9).
- `FaceSelection` is built only through three constructors; the zero value
is rejected.

**Security-Sensitive Operations:**

- Identity-anchor change (D5): the inheritance closure follows
identity-scoped edges at entity level only. The PR states the access delta and
gets a `rela-security-reviewer` pass.
- EndpointMatch (D5, A6). Open: RR-QUXMAF on ACL gate arms under a raw
selection.
- Relation writes on faced sources (D4): identity edges need every face;
content edges need their tail face; `relation_grants` do not bypass that.
- Trace and analyze output: face lists and orphan decisions are computed
after row gating (A4), so hidden faces and edges are not disclosed.
- ACL principal lookup and traversal keep today's behaviour (A5).
- A hidden entity stays a uniform 404; no new error names a face the caller
cannot read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 to AC7: storetest groups in design 6.1 (address, selection, family
operations, attachments, relations, history, graph differential, budget, plans),
run on all four backends; versions on pg and sqlite.
- AC8 and AC9: design 6.3 regression tests through CLI, MCP, Lua and the
data-entry API.
- AC10: archguard tests.

**Edge Cases:**

- `AtFaces()` with no faces matches nothing.
- `FaceIn` composes with each selection mode; an ignored `FaceIn` would
fail open, so it is pinned.
- `AllFaces` pages resume mid-family.
- `DeleteFace` of the last face leaves no entity; inbound edges remain as
with `DeleteFamily(id, false)` (A3).
- `DeleteFace(Ref{id, ""})` on a faceless entity deletes it.
- A family that exists only at named faces counts for `HighestID` and for
`AttachFamilyFile`.
- Concurrency: `DeleteEntityState` racing a new face create (pg, A2);
manager-level delete racing an update (design 6.3), under `-race`.
- A faced endpoint type that lacks the query's `AtFaces` faces does not
match (A6).

**Negative Tests:**

- Zero `FaceSelection` returns `ErrInvalidQuery` from every read method.
- Zero Ref, `Ref{ID: "X@draft"}`, `Ref{ID, ""}` on a faced family and
malformed faces return `ErrNotFound`.
- `AttachFamilyFile` with no face of the id returns `ErrNotFound`.
- A principal without write on every face cannot create an identity edge
from a faced source; `relation_grants` do not satisfy a content edge on an
unwritable tail.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. User-visible behaviour: analyze reports grow and trace shows faced nodes
without titles until Stage 3. Mitigation: report headers state coverage.
2. `AllFaces` costs more in whole-store scans. Mitigation: bounded by
declared faces; Counting budget tests guard per-page paths.
3. Runtime `ErrInvalidQuery` in an untested path. Mitigation: the literal
guard and storetest.
4. Migration 0018 blocks writes during the index build. Mitigation: release
notes advise a maintenance window for large tables.
5. Migration number collision with `develop`. Mitigation: check at merge
(0017 is the highest today).
6. Merge conflicts between PRs 6 to 8. Mitigation: serial merge.
7. Access delta from D5. Mitigation: conservative ruling, security review,
and RR-QUXMAF open for the coordinator.

Effort: xl.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal store API; the only user-visible changes are analyze/trace output and a release note on the 0018 lock, which the PRs carry)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- Release note: 0018 index build lock (PR 1).
- `docs/postgres-backend.md`: migration 0018 (PR 1).
- CLAUDE.md: the "Don't add a zero-face read" rule is replaced by the
`bareref` guard rule (PR 6).
- Godoc on `store.Store`, `FaceSelection` and `FaceIn`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan, except RR-QUXMAF, which refines ruling D5 and stays open for the coordinator

**Design Review Findings:**

- Significant, addressed: RR-FYAKXX, RR-WGW5D8, RR-VN71BT, RR-HBOKY4,
RR-EMH0AC.
- Significant, open (conflicts with ruling D5): RR-QUXMAF.
- Minor, addressed: RR-CRQ183, RR-DYURK3, RR-6NZ4YD, RR-F1XMQ2, RR-P90GOX.
- Minor, wont-fix: RR-Z4QBXC.
- Nit, addressed: RR-O0E6WK.
