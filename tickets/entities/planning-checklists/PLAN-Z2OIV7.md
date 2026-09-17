---
id: PLAN-Z2OIV7
type: planning-checklist
title: 'Planning: Read-gate scope command payloads (entity/list), then reconsider the view-context deferral'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **RE-SCOPED AFTER DESIGN REVIEW (2026-07-27).** Design review (RR-3T18K9,
> RR-WAE2E4) proved the view-traversal fix is a shared read-side ACL bug
> affecting the `_views` API and entity-detail sections, not just commands. It
> was **split out to BUG-9Z20WH**. This ticket now:
> - **depends-on BUG-9Z20WH** for view-command support;
> - scopes **entity + list** command payloads (independent, can proceed);
> - **consumes** the gated traversal for view (no command-local traversal
>   gating) once BUG-9Z20WH lands.
>
> Blocking decisions D1/D2/D3 stand; the executeView mechanism moved to the bug.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** route command stdin-payload builders through the ACL read seam.
**Entity + list here.** View gating lives in BUG-9Z20WH; this ticket consumes
its result and lifts the `context: view` deferral once it lands.

## Decisions (2026-07-25, unchanged)

- **D1 — `entity_type` is a HARD prerequisite** (missing → 400, no ungated
fallback). The frontend `entity_type` add lands in this PR (RR-TXT57E: 3 edits
across 2 files + a test, not "one line"; verified `CommandModal.vue` is the only
frontend `/api/command/` caller).
- **D2 — adopt the list's configured sort.** Payload changes even under NopACL
for lists; the NopACL regression asserts *list-sorted* output.
- **D3 — lift the view deferral** — but only *after* BUG-9Z20WH gates the
traversal. This ticket removes the `authorizeCommand` view deny-arm and adds
view-command payload consumption; it does NOT gate the traversal itself.

## Research

- [x] Searched for existing libraries — N/A, internal seam
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Prior art (verified):** `visibleReader.getVisible` (`visiblereader.go:57`),
`App.scopedSortedEntities` (`api_v1.go:282`), `visibleRelationIDs`
(`relation_visibility.go:67`), `filterVisible`. **`export.go` is the in-package
precedent** for wiring these into a non-`*App` handler via closures. All read
verdicts resolve through the same `readQuery` — consistent gating by
construction (design review question 5, confirmed).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Handler wiring:** add closures to `commandHandler`, wired in `app.go` per the
`export.go` precedent — `getVisibleEntity`, `scopedEntities`, `visibleRelations`
(wrapping `relationsForEntity` + `visibleRelationIDs`), `resolveEntityType` is
NOT needed (D1 removed the fallback).

**Entity context:**
1. `entity_type` from query; **absent → 400 BEFORE any store read** (D1,
RR-TXT57E: the early return keeps the error from being an oracle).
2. `getVisibleEntity(ctx, type, id)`; `(nil,false,nil)` → 404 (same as missing).
3. Relations → `visibleRelations` closure (drop hidden neighbors).

**List context (RR-WWM5XL):** `scopedEntities(ctx, listCfg.EntityType, nil)` for
ACL scope + the list sort (D2), **then `applyFilters(entities, listCfg.Filters)`
after** — committed choice, NOT "translate into query." Document that
`$`-variable filters are skipped in command payloads (no request context to
substitute from — matches today's behavior but becomes security-relevant once
the payload is permission-gated: scope such commands via ACL, not the list
filter).

**View context — DEPENDS ON BUG-9Z20WH:** once the traversal is source-gated,
`buildViewInput` consumes an already-gated `vr` — no command-local filtering
needed. This ticket then: (a) removes the `authorizeCommand` view deny-arm so
`context: view` is permission-gated like entity/list (RR-TXT57E: the ReadOnlyACL
and default-deny arms are structurally upstream, so removing the view arm opens
no hole — verified); (b) **inverts** the existing
`TestCommandExecDeclarativeFailsClosed` "view denied despite granted permission"
case to assert 200; (c) updates the stale in-code godoc documenting the deferral
(`commands.go` authorizeCommand block + view-arm comment).

**Alternatives considered:**
- *Hand-roll `PermitsRead`/`ReadQuery`.* Rejected: duplicates the seam.
- *400-less entity fallback.* Rejected by D1.
- *Command-local view traversal gating (option a-local / b).* Rejected:
option (b) leaks (RR-3T18K9); the correct fix is shared and became BUG-9Z20WH.

**Files to modify:**
- `internal/dataentry/command_handler.go` — closures
- `internal/dataentry/app.go` — wire them
- `internal/dataentry/commands.go` — entity/list builders + exec switch + `authorizeCommand` view-arm removal + godoc
- `frontend/src/components/entity/CommandModal.vue` + `EntityDetail.vue` + `CommandModal.test.ts` — send `entity_type` (RR-TXT57E: 3 edits + test)
- `internal/dataentry/commands_test.go` — new + inverted tests
- `docs-project/entities/guides/GUIDE-acl-security.md`, `GUIDE-data-entry.md` — scoped-payload table, view now grantable, migration notes; `just docs` (NEVER edit generated files)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

- **No-side-channel closed for entity** (D1, no fallback): `entity_type`
mismatch → gate denies → 404; the 400 for a missing type returns before any
store read.
- **View safety inherited from BUG-9Z20WH**, not re-implemented here. This
ticket must NOT lift the view deferral until BUG-9Z20WH has landed — the
`depends-on` enforces ordering.
- **List DenyAll → empty**, not an error that leaks type existence.
- **`$`-filter caveat** (RR-WWM5XL): documented; ACL is the scoping mechanism,
not the skipped list filter.

## Test Plan

- [x] Test scenarios per acceptance criterion
- [x] Edge cases identified
- [x] Negative tests defined
- [x] Integration approach defined

Go, `httptest`, alongside `commands_test.go`.

| Scenario | Expect |
|---|---|
| entity cmd, `entity_type` sent, may read | 200, entity in payload |
| entity cmd, may NOT read | 404 (= nonexistent) |
| entity cmd, `entity_type` ABSENT | **400 before any store read** (D1) |
| entity cmd, hidden neighbor | neighbor absent from `Relations` |
| list cmd, mixed readable/hidden | only readable rows |
| list cmd, `listCfg.Filters` set | filters still applied (after scope) |
| list cmd, `$`-variable filter | filter skipped (documented), ACL-scoped only |
| list cmd, NopACL | payload = **list-sorted** (D2) |
| list cmd, DenyAll | empty entities, no error |
| view cmd under policy, permission granted | 200 (**after BUG-9Z20WH**; inverts the old deferral test) |
| view cmd, hidden intermediary/neighbor | absent (**inherited from BUG-9Z20WH**) |
| NopACL, entity | byte-identical to today |

**Edge cases:** empty list/collection → `[]` not `null`; `entity_type`
mismatched to real type → 404; view tests gated behind BUG-9Z20WH landing.

## Risk Assessment

- [x] Technical risks + mitigations
- [x] Security risks (see above)
- [x] Effort estimated

**Risks:**

1. **Ordering vs BUG-9Z20WH** — lifting the view deferral before the traversal
is gated would ship the leak. *Mitigation:* `depends-on BUG-9Z20WH`; the
view-deny-arm removal + inverted test only make sense once it lands. If
BUG-9Z20WH slips, ship entity+list here and leave view deferred (D3 becomes a
follow-up on this ticket).
2. **D1 hard break for direct API callers** — a non-browser POST without
`entity_type` → 400. Accepted cost of no side channel; loud migration note.
3. **List reorder (D2)** — payloads change under NopACL. *Mitigation:*
migration note; tests assert list-sorted output.
4. **`authorizeCommand` view-arm removal** — must not open view under
`--read-only` or NopACL incorrectly. *Mitigation:* verified structurally safe
(RR-TXT57E); existing ReadOnly/NopACL view subtests stay green.

**Effort:** **`m`** — reduced from `l` now that the executeView work moved to
BUG-9Z20WH. This ticket is entity/list scoping + wiring + the view-arm flip.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist created at implementation

- [x] `docs/acl-security.md` — scoped-payload table; `context: view` grantable
(after BUG-9Z20WH); the `$`-filter caveat.
- [x] `docs/data-entry.md` — migration note: entity commands REQUIRE
`entity_type` (direct API callers break without it); list payloads may reorder;
`$`-variable list filters are not applied to command payloads.
- Edit `docs-project/entities/guides/`, regenerate with `just docs`.

## Design Review

- [x] Run `/design-review` before implementation
- [x] All critical/significant findings addressed

**Findings:** RR-3T18K9 (critical — option b leaks) → resolved by splitting the
traversal fix to BUG-9Z20WH and depending on it. RR-WAE2E4 (significant — leak
is live in _views/sections) → BUG-9Z20WH. RR-WWM5XL (significant — list filter
porting) → "apply after" committed, `$`-caveat documented. RR-TXT57E (minor —
frontend scope + invert view tests + godoc) → folded into Approach/Files.

**Resolved open questions:**
1. executeView boundary → **option (a), split to BUG-9Z20WH** (user decision).
2. D1 direct-API-caller break → **accepted**, loud migration note (user: hard
prerequisite).
3. Frontend dependency → **folded into this ticket** (the `entity_type` add).
