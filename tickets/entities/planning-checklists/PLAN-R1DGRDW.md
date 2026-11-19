---
id: PLAN-R1DGRDW
type: planning-checklist
title: 'Planning: Point-in-time (as-of) reads of a world'
started: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `?as_of=` on data-entry reads (lists, kanban, calendar, gantt,
dashboard cards, entity detail and views, documents, search) on pgstore and
sqlitestore; read-only enforcement; SPA date picker and banner; docs guide with
an SoA audit example. Out: fs and memory backends (refuse with
`as_of_backend_unsupported`); CLI and MCP as-of reads; historic schema or ACL.

**Acceptance Criteria:**
1. A list at T shows values at T and omits rows created after T (TestAsOf_SQLite_ListReadsTheInstant).
2. Detail at T 404s for an entity created after T (TestAsOf_SQLite_DetailReadsTheInstant).
3. Every write under as_of is 422 `as_of_read_only` (TestAsOf_SQLite_WritesAreRefused).
4. Search, documents and dashboard cards answer at T (TestAsOf_SQLite_*).
5. Deleted-after-T and face-published-after-T are handled in the store (storetest.RunInstantTests on pg and sqlite).
6. memstore refuses with 501 (TestAsOf_RefusesWhatItCannotServe).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-9IOGBV3

**Existing Solutions:** Temporal tables (SQL:2011 `FOR SYSTEM_TIME AS OF`) are
not in PostgreSQL or SQLite. The existing version tables already hold every
state, so a CTE that shadows `entities`/`relations` at the query seam reuses
every read path unchanged. The world middleware (`attachWorld`,
`worldCapablePath`) is the pattern for the request edge.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** The instant is bound on ctx (`store.BindReadInstant`).
pgstore and sqlitestore rewrite reads through a CTE that picks, per row, the
live row when it is not newer than T, else the latest version at or before T,
and drops deleted, purged, superseded or soft-deleted rows. Writes under an
instant return `ErrReadOnlyInstant`. Data-entry middleware `attachAsOf`
validates the parameter and admits routes one by one. Rejected: per-handler
history reads (duplicates every pipeline); a Go-side snapshot (unpaged, breaks
collection-reads).

**Files to modify:** internal/store (asof.go, storeutil, pgstore, sqlitestore,
storetest), internal/sqlitedb, internal/dataentry (asof.go, world.go,
document.go, api_v1.go), internal/lua/cache.go, internal/search/index.go,
internal/apiwire/v1, frontend (api, composables, components/common, router,
stores, i18n), docs-project guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `?as_of=` only: RFC 3339 with zone, given once;
routes are an allowlist (`asOfCapablePath`).

**Security-Sensitive Operations:** History reads go through the same visibility
wrappers with today's ACL (DEC-TDEF3GD). Caches are bypassed or keyed by the
instant so a past render is never served as today's or across instants.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; store conformance in
storetest.RunInstantTests on pg and sqlite.

**Edge Cases:** Created after T, deleted after T, renamed or face-converted,
face published after T, soft delete, purge, rows without history.

**Negative Tests:** Malformed, zone-less and duplicated `as_of`; writes;
unsupported routes; backends without history.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A read path that bypasses the store would answer with today's data;
mitigated by the route allowlist and refusing everything else. Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes (new guide GUIDE-point-in-time-reads)
- [x] CLAUDE.md - New patterns or conventions (.claude/rules/versioning.md)
- [x] README.md - Project-level changes
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Design settled in RES-9IOGBV3 and DEC-TDEF3GD; code
findings are tracked as review-responses on the ticket.
