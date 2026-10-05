---
id: PLAN-Y1JVSB
type: planning-checklist
title: 'Planning: entity.Ref and one gated resolver in internal/visibility'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (Stage 1 of RES-Y6JA37 / DEC-NPZICR):

- `entity.Ref{ID, Face}` with a text codec; it replaces `dataentry.rowKey`, `mcp.hitKey` and `pgstore.stateKey`.
- One resolver in `internal/visibility` with `Ref`, `InWorld`, `Address` and `Family` modes, plus the batched `EndpointsReadable` for `FilterRelations` (ruling 3).
- Migration of the dataentry, mcp and lua single-entity read-out paths onto it, and deletion of `entityRef`, `parseEntityRef`, `bareEntityID`, `getVisibleRef`/`getEntityRef` + `faceReadable` pairs and `visibility.parseAddress`.
- Attachments on a per-face model (bytes per entity, value per face), including the fs/mem `AttachFile` existence check (ruling 8) and the file-property write rule (design 8.1).
- History/restore/purge on `Ref` (HTTP and CLI) and the frontend `servedRef` URLs.
- Bugs fixed on top: BUG-CTUW2N, BUG-BZQQDP, BUG-4SYAA6, BUG-FYEEVX (BUG-8J3LSB already merged in Stage 0).

Out of scope:

- Bare ids on faced types in MCP and Lua keep missing until Stage 3 (ruling 1, TKT-7IZHP0).
- Store API taking `Ref`, invalid zero `EntityQuery`, attachment method renames, per-face byte keys, `DeleteRelation` with `FromFace` (Stage 2).
- CalDAV alias ids (BUG-7MB1D5) and `analyze.go:268,616` (BUG-95W7MV), per ruling 6.
- The N+1 neighbour reads in `export.go:387` and `export_list.go:481`.

**Acceptance Criteria:**

1. No dataentry, mcp or lua READ-OUT path calls the store directly for a single entity; write-prep reads stay raw (ruling 7). Scenario: the extended archguard test (design 8.5) passes with an allowlist that holds only write-prep entries, each with a reason.
2. A handler cannot obtain a row without the row gate and face gate having run. Scenario: resolver unit tests over faced, faceless and world fixtures; a principal granted `type@a` only gets a 404 for `ID@b` on GET, history, comments, views, actions, relations and attachments.
3. Denied, missing, type-mismatched, face-denied, world-denied and load-failed reads are indistinguishable (ruling 2). Scenario: parity test against the old `getVisibleRef` for every gate branch; a failing loader yields a miss and one `slog.Warn`.
4. Attachments work per face (BUG-CTUW2N). Scenario: upload, list, download and delete on `ID@draft` and `ID@published`; a face never lists or downloads another face's file; bytes are deleted when the last face drops the name.
5. A writer cannot reference another face's file through an ordinary write (RR-EV48RR). Scenario: PATCH of a file-type property returns 422; the cross-face download attempt is a 404.
6. Relation reads gate a content-scoped tail at its face (RR-2IK76Z). Scenario: a relation whose tail is on a hidden face is a 404 on GET and absent from `FilterRelations` output.
7. History and restore work per face (BUG-4SYAA6). Scenario: `GET /history` and restore on `ID@face`, and CLI `history ID@face`.
8. Frontend sub-resource URLs use the served face (BUG-FYEEVX). Scenario: e2e on the faced fixture (TKT-WCMW47) opens attachments, history and comments on a non-first face.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal addressing and ACL design; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Y6JA37 (option C), decided by DEC-NPZICR.

**Existing Solutions:**

- Codebase: `ParseStateRef`/`FormatStateRef` (entity/face.go) as the codec; `visibility.PolicyReader`/`AllowAllReader` and the `search.VisibleSearcher` decorator pattern (DEC-ZBI39P); `store.EntityQuery.World`/`FaceIn` for world ranking; `acl/ceilingguard_test.go` and `archguard/zeroface_test.go` as guard-test precedents; `lock.For(st)` for the attachment lock.
- Other projects (in RES-Y6JA37): Drupal's "not specified" language code (implicit face), Strapi v5 `documentId + locale + status` (two-level addressing), Sanity perspectives (worlds).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Full design: `.ignored/stage1-design.md`, sections 1 to 8. Section 7 (rulings)
and section 8 (design-review amendments) win over earlier sections.

- `entity.Ref` is a comparable struct with `ParseRef`, `String`, `IsZero`, `MarshalText`/`UnmarshalText` and `(*Entity).Ref()`. Wire form unchanged.
- `visibility.Resolver` is one concrete type built by `NewResolver(gate, redact, load)` or `NewAllowAllResolver(load)`, both rejecting nil. Gate order: world denied → row gate on bare id → load → stored type equals claimed type → face gate → redact once. A load failure is a miss plus `slog.Warn`; only gate failures return an error.
- `Family` is a header read returning readable faces sorted by token. `EndpointsReadable` batches per type, heads by `Family`, content-scoped tails by `(id, face)`.
- World mode: default scope reads `""`; other scopes use `ListEntities{IDs, World, FaceIn}` with the permitted-face set, where "no face" is a miss before the query.
- Scripts and MCP learn the stored type with one raw header read, then call the typed resolver.
- Attachments: download requires the name in the addressed face's value; upload/delete/copy/single-face delete run the reference count under the `(entity, property)` lock; generic writes may not change file-type values.
- PR sequence: 1 Ref; 2 resolver; 3 dataentry + guard; 4 attachments; 5 MCP + Lua + `FilterRelations`; 6 history; 7 frontend (design 8.9).

Alternatives rejected: a mode enum on one method (consumers want one or two
methods; consumer-side interface rule); embedding `Ref` in `WorldCandidate`
(churns every backend); an untyped resolver that gates on the loaded type (two
load orders); gc-based byte deletion (ruling 4); surfacing load errors as 500
(ruling 2); per-face byte keys now (store API, Stage 2).

**Files to modify:**

- `internal/entity/face.go` (Ref), `internal/store/world.go` (`WorldScope.RuleAt`, `WorldCandidate.Ref`), `internal/store/{memstore,fsstore}` attachment existence check + storetest case.
- `internal/visibility/resolver.go` (new), `policyreader.go`, `allowall.go`, `scriptreader.go`, `visibility.go`.
- `internal/dataentry`: `visiblereader.go`, `entityreader.go`, `entityref.go` (deleted), `api_v1.go`, `handlers_attachment.go`, `export.go`, `write_handler.go`, `relation_read_handler.go`, `relation_history_handler.go`, `relations_modern.go`, `history_handler.go`, `history_restore.go`, `comments_handler.go`, `views_handler.go`, `viewworld.go`, `actions.go`, `commands.go`, `detailactions.go`, `affordances.go`, `document.go`, `gantt_handler.go`, `mentions.go`, `app.go`, `worldneighbors.go`.
- `internal/attachment/attachment.go`, `internal/entitymanager` (file-property write rule, `StampAttachments`, copy-engine lock, single-face delete ref count), `internal/cli` (attach/detach/history/restore/history-purge `ID@face`).
- `internal/mcp`: `tools_entity.go`, `tools_trace.go`, `convert.go`, `prompts.go`, `resources.go`, `tools_analysis.go`, `tools_attachment.go`, stdio wiring.
- `internal/lua`: `runtime.go`, `elevation.go`, `deps.go` (doc).
- `internal/archguard/zeroface_test.go` (extended guard), `frontend/src` sub-resource URLs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- HTTP path segments and MCP/Lua addresses (`ID` or `ID@face`): parsed by `ParseRef` / `ParseStateRef` grammar; a parse failure is the uniform miss, never a fallback read.
- Attachment file names: base name only (`path.Base`), must be a member of the addressed face's value (allowlist); stored directory prefixes are not trusted.
- File-type property values in writes: rejected (422) unless unchanged; only trusted attachment paths set them.
- World selection: from the request's world handle or wiring; a denied world is a miss.

**Security-Sensitive Operations:**

- Row gate and face gate on every single-entity read-out (resolver; guard test enforces).
- Field redaction exactly once per row (`PrimeTraversals` + `Redact`).
- Attachment byte download (face membership check) and deletion (reference count under lock).
- Relation endpoint gating (tail at its face).
- Allow-all capability only via `NewAllowAllResolver` at wiring, never inferred from identity.
- Errors: every denial is an indistinguishable 404; load failures log server-side without property values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: extended archguard test; allowlist entries carry reasons.
2. AC2: table-driven resolver tests (Ref/InWorld/Address/Family × faced/faceless/world × granted/denied); HTTP tests per migrated route on the faced fixture.
3. AC3: parity test vs. `getVisibleRef`; failing-loader test asserts miss + warn.
4. AC4: attachment service and HTTP tests per face; storetest case for `AttachFile` on a faced id (fs/mem/pg/sqlite).
5. AC5: PATCH file-type property → 422; cross-face download → 404; restore leaves file values.
6. AC6: relation GET and `FilterRelations` with a tail on a hidden face.
7. AC7: history HTTP + CLI per face.
8. AC8: e2e specs on the faced fixture.

**Edge Cases:**

- `ParseRef("")`, `"ID@"`, `"@face"`, zero `Ref` marshalling, JSON map-key round trip.
- Claimed type differs from stored type → miss.
- Principal with no readable face on a faced type → miss, no store query.
- Faceless type in a non-default world resolves to `""`.
- Family with only hidden faces → miss; family order deterministic.
- Upload name collision with a hidden face's file; max == 1 replacement.
- Copy racing a delete of the last reference (under `-race`).
- Single-face delete removes unreferenced bytes only.

**Negative Tests:**

- Unparseable address, unknown face, denied world, denied type, denied face: each a 404 identical to a missing id.
- Generic write that adds or changes a file-type value: 422.
- Download of an orphan file no face references: 404 (B2).
- CLI attach with a bare id on a faced type: refused, message names the faces.

Integration: `storetest` conformance on all four backends, `storetest.Counting`
budget for `EndpointsReadable` (10 vs 50 relations), e2e on the TKT-WCMW47 faced
fixture, `just test-postgres`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Wide migration (about 60 call sites across three packages): mitigated by seven independently green PRs and the guard test that fails on any missed site.
- Behaviour change B2 (orphan files undownloadable): accepted by ruling 5.
- File-property write rule may break a script or flow that sets file values directly: SPA duplicate already omits them; the 422 message points to the attachments API.
- Copy-engine lock adds a dependency to entitymanager: a consumer-side interface, nil rejected at construction.
- Upload suffixing oracle on hidden faces: deferred to Stage 2 (RR-1JGEEO).
- Extra header query per scripted get: one indexed query; accepted.
- Bare ids on faced types in MCP/Lua still miss until Stage 3 (ruling 1).

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- `docs/content-states.md`: per-face attachments, file-property write rule, relation tail gating.
- `docs/cli-reference.md`: `attach`/`detach`/`history`/`restore`/`history-purge` accept `ID@face`.
- `docs/acl-security.md`: one resolver for single-entity reads; uniform miss for load failures.
- `CLAUDE.md`: the resolver as the only single-entity read-out path; the extended guard.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-EV48RR (critical, addressed), RR-2IK76Z,
RR-FE1EGP, RR-0SD5ER, RR-TG5ZBC, RR-Z23T2T, RR-S4S8ZG (significant, addressed),
RR-12WFBV (minor, addressed), RR-1JGEEO (minor, deferred to Stage 2), RR-W9PIZB
(nit, addressed). Amendments are in `.ignored/stage1-design.md` section 8.
