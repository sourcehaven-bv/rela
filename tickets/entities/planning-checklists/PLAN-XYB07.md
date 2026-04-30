---
id: PLAN-XYB07
type: planning-checklist
title: 'Planning: Add search interface to data-entry list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A search input above every list view at `/list/:id` that filters the rows by free-text against id, title, and entity content.
- A `+ Filter` dropdown next to the search input that lets the user add ad-hoc filters on any property of the list's entity type.
- Backend `q=` parameter on `GET /api/v1/{plural}` that intersects free-text-search hits with the typed list (preserving list sort, pagination, and pre-configured filters from `data-entry.yaml`).
- URL deep-linking: `q` and ad-hoc `filter[prop]` survive reload + back/forward.
- Keyboard: `/` focuses the search box; `Esc` clears it.

Out of scope:
- Saved searches / named queries.
- New operators beyond what `filterStateToApiParams` already serializes.
- Replacing or merging the standalone `/search` route.
- Cross-type search on a list view (a list is always one type).
- Per-property operator selection in the ad-hoc filter dropdown — v1 uses `=` only.

**Acceptance Criteria:** see ticket TKT-603FQ for the AC1..AC10 list. Each was
verified during review (REV-2GHLY).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

Reused: `SearchView.vue`'s filter dropdown (lifted into `AdHocFilterMenu`),
`useUrlFilterSync` (extended with `q`), `filterStateToApiParams`,
`runFreeTextSearch`. No third-party library needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

Per user direction: backend `q=` (not frontend-only), explicit `+ Filter`
dropdown matching SearchView's pattern, SearchView refactored to consume the
shared component (no duplicated dropdown code), e2e skipped.

Implementation lives across `internal/dataentry/{helpers,api_v1,app}.go`
(backend) and
`frontend/src/components/lists/{SearchBox,AdHocFilterMenu,EntityList}.vue`,
`frontend/src/composables/{useUrlFilterSync,useListKeyboard,useKeyboardShortcuts}.ts`,
`frontend/src/views/SearchView.vue`,
`frontend/src/components/common/Sidebar.vue`. See IMPL-HCWVA for the full file
list.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

`q` flows through `searchparser.ParseQuery` (same path as the existing
`/api/v1/_search` endpoint). `filter[prop]` ad-hoc params reuse
`parseFilterQueryParams`'s `PROPERTY_NAME_RE` allowlist. Property names in the
menu come from the schema, never user input. No new security-sensitive
operations.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Test coverage delivered:
- Backend: `TestV1ListEntitiesSearchQuery` with 10 sub-tests (empty q, intersection + sort preservation, no matches, AND-combine with filter, whitespace q, prop-only q, type pinning, error → 500, q + pagination, quoted phrase forwarding).
- Frontend: `SearchBox.test.ts` (6 tests), `AdHocFilterMenu.test.ts` (6 tests including the C4 regression), `EntityList.test.ts` integration block (AC2/AC3/AC4).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

Effort: m. Identified risks (Bleve ordering, debounce coalescing, menu/FilterBar
collisions, scope navigation `q` forwarding) all mitigated as documented.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~CLI help text~~ (N/A: no command changes)
- [x] ~~CLAUDE.md~~ (N/A: no new architectural pattern)
- [x] ~~README.md~~ (N/A: no project-level surface)
- [x] ~~User guide / reference~~ (N/A: project does not maintain a separate data-entry user guide; the in-app affordance is self-documenting)
- [x] ~~N/A toggle~~ (N/A: superseded by per-line N/A annotations above; see DOCS-92T0E)

`docs-checklist` DOCS-92T0E created and marked done.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: planning was reviewed and approved by the user before implementation; explicit `/design-review` command not used. The `cranky-code-reviewer` agent surfaced design-level findings (12 in total) post-implementation, all addressed or deferred with reason — see REV-2GHLY)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: addressed in implementation phase via review-response entities — see REV-2GHLY for the table)

**Design Review Findings:** addressed via post-implementation code review (see
REV-2GHLY for review-response IDs RR-W5DGH, RR-32RJA, RR-NG9Y2, RR-JW7GG,
RR-O3LD9, RR-OCKXX, RR-C9TH7, RR-H8X20, RR-C9ZEF, RR-SILQH, RR-I5WU0, RR-YI5PQ).
