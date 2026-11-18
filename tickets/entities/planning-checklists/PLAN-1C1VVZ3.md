---
id: PLAN-1C1VVZ3
type: planning-checklist
title: 'Planning: Audit read API and Changes tab in data-entry'
started: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: newest-first paged reads in sqlaudit (per entity id through
audit_subjects, and global); a visibility audit reader; GET
/api/v1/_changes/{type}/{id} and GET /api/v1/_audit; permission `audit:read`;
`app.audit_enabled` on /_config and a per-principal `audit_readable` on the
sidebar; a Changes panel on the entity page and an /audit page. Out: filters by
op, principal or time (no index yet); following an entity across a rename; the
fs tier (JSONL has no query API, so nothing is offered there).
<!-- Document explicitly what IS and IS NOT in scope -->

**Acceptance Criteria:**
1. On sqlite and postgres, GET /_changes/{type}/{id} returns that entity's write records newest first, paged by cursor (limit default 50, max 200).
2. A caller who cannot read the entity, or cannot read every live face of its family, gets the same 404 as for a missing entity. A deleted entity needs `history:read`, as history does.
3. The Changes list leaves out denied-write, acl-bypass and other non-write ops, and drops a relation or rename record that names an entity the caller cannot read.
4. GET /_audit returns every record newest first for a holder of `audit:read` under a configured policy, including deleted entities and denied writes; 403 naming the permission otherwise (no-policy and read-only deny).
5. On the fs tier both routes answer 404 and `app.audit_enabled` is false; the UI shows neither.
6. A budget test shows the per-page store cost of /_changes does not grow with the number of records or entities.
<!-- Each criterion must have a concrete test scenario -->
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-HVW3B7O covers it, step 3)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reads over the existing sqlaudit tables)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows the in-repo history read path)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-HVW3B7O

**Existing Solutions:**
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** (revised after design review)
- `audit.Entry{Seq, Record}` in the audit package. `audit.Subject` gains `Face` (the entity's face; on a relation, its From face), set by every entity and relation producer.
- New pg migration and sqlite rung: `audit_subjects` gains `face` and `starts`. `starts` marks a record that begins a lifecycle of that id and face: `create-entity` with that subject, or `rename-entity`/`convert-entity` whose After is that id. Backfilled from `doc`. A relation's To end is stored with face ''.
- sqlaudit: `Newest(before, limit)` (global) and `ForEntity(id, face, ops, before, limit)`: one query for the fence (latest `starts` row for id and face), one page query `s.entity_id = ? AND s.face IN (face, '') AND s.seq >= fence AND s.seq < before AND l.op IN (ops)` newest first.
- Change ops (allowlist, everything else excluded): create-entity, update-entity, delete-entity, rename-entity, convert-entity, restore-entity, purge-deleted-entity, copy-state, attach-render, create-relation, update-relation, delete-relation. A test classifies every `audit.Op*` constant.
- internal/visibility `AuditReader.EntityChanges`: per page, drop records whose Kind is not entity or relation, entity records of another type, and records naming another id (relation end, rename Before/After) that one batched `Resolver.ReadableTypes` call per page says is hidden. It returns a `ChangeRecord` DTO: seq, time, op, subject, before/after, principal user and tool, summary only for ops whose summary is id-free, trigger kind with ids removed.
- Global: `AuditReader.Newest` unfiltered; `/_audit` checks `access.ModeOf(acl).PermitsElevatedRead(ctx, gate, acl.PermAuditRead)` (no policy and read-only deny), and records one `audit-read` record per request (principal, cursor, count).
- dataentry: `/_changes/{type}/{id}` resolves one face exactly as history does (historySubjectOr404) and reads that face. Wired by a dataentrywire free function; nil reader means 404 and `audit_enabled` false.
- Frontend: ChangesPanel on EntityDetail and AuditView at /audit, linked when the sidebar says `audit_readable`; text interpolation only.
Alternatives rejected: per-row ACL callback (forbidden on list paths); Go-side
op filter after LIMIT (attacker-floodable); fence computed by scanning an
entity's records in Go (cost grows with history); requiring every face readable
(oracle).

**Files to modify:** internal/audit (Entry), internal/audit/sqlaudit
(+sqlaudittest), internal/visibility/audit.go, internal/acl/policy.go,
internal/dataentry (handlers, config, sidebar), internal/apiwire/v1,
internal/dataentrywire, cmd wiring, frontend (api, panel, view, router,
sidebar), docs-project guides.
<!-- List specific files that will change -->

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Path type/id (resolved through the existing
address resolver), `limit` (int 1..200), `cursor` (opaque base64 of a positive
int64 seq; anything else is 400). All SQL is parameterized.
<!-- For each input: source, validation approach, what happens on invalid input -->

**Security-Sensitive Operations:** Row-level read of audit records: entity
existence is secret, so every refusal on /_changes is the uniform 404 and hidden
endpoints drop records without a count. The global view is an elevated read
behind a config-declared permission, so it answers 403 naming `audit:read`
(config is not secret). Records carry property names, never values.
<!-- List operations and how they're protected -->

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 sqlaudittest paging (both backends) and handler test;
AC2 handler tests with a real Declarative policy: hidden row, hidden sibling
face, deleted without history:read, all 404 identical to missing; AC3 visibility
tests with relation to a hidden entity and denied-write records; AC4 handler
tests for holder, non-holder (403), no-policy and read-only (403); AC5 fs-tier
wiring test; AC6 budget test in querybudget style plus a counting fake of the
audit reads.
<!-- Map each acceptance criterion to how it will be tested -->

**Edge Cases:** Entity with no records (empty list, no cursor); record with nil
Subject (not indexed, never in /_changes); rename records (Before/After ids both
indexed); relation whose far end is deleted (gate as hidden unless readable);
faced type with a hidden sibling face (404); limit larger than remaining rows; a
cursor from another entity (only bounds seq, harmless).
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:** Bad cursor, limit 0 / 201 / non-number, unknown type, hidden
entity, global without permission.
<!-- What should fail? How should it fail? -->

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A page whose records all name hidden entities comes back short or
empty with a next cursor; acceptable, and it reveals no count. Rename lineage is
not followed: records under the old id stay under it (documented). Effort: l.
<!-- List risks and how they will be mitigated -->

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [ ] docs/metamodel.md - New metamodel features
- [ ] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes (and GUIDE-audit-log, GUIDE-acl-security for audit:read)
- [ ] CLAUDE.md - New patterns or conventions
- [ ] README.md - Project-level changes
- [ ] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the has-review-response links on TKT-X1ZFO04
(rela-security-reviewer, 4 significant, 3 minor, 2 nit; all addressed in this
plan).
