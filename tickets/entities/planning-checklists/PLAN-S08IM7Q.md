---
id: PLAN-S08IM7Q
type: planning-checklist
title: 'Planning: Bound the structured search path the way free-text already is'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `/api/v1/_search` returns at most one page of rows on every
branch. The page size is `limit` when set, else the shared bound (today
`maxFreeTextSearchResults` = 1000, renamed `maxSearchResults`). A new `page`
parameter reads later pages. `meta.total` is the post-gate match count,
`meta.per_page` the page size, `meta.has_more` whether rows remain after this
page. `searchPlan.page` always sets `GraphQuery.Limit` (and `Offset` for later
pages), so its allowlist entry goes. SPA callers that need every row (dashboard
query cards shown as breakdown or table) page through `/_search`; a count card
keeps its request and reads `meta.total`. The search page says when it shows
only the first page. Out: the Go fallback for shapes the plan declines still
reads the whole set before cutting (TKT-PSBGQLY, TKT-C5H506O). Server-side
aggregation of query cards (TKT-AIEGHU). Paging the free-text relevance cap
beyond 1000 hits.

**Acceptance Criteria:**
1. Structured `/_search` without `limit` over more than 1000 matching rows returns 1000 rows, `total` = every visible match, `has_more` = true. Test: seed 1000+ tasks, call handler.
2. `page=2` returns the next rows in the same order, no row twice, across several types. Test: differential test pages through plan and Go path and compares the concatenation to the full Go result.
3. Hidden rows never count toward `total`, `has_more`, or the offset. Test: restricted principal paging case in searchplan_test.
4. Invalid `page` (0, negative, non-integer, too large) answers 400 `invalid_page`.
5. Unpaged-read allowlist entry for `searchPlan.page` is removed and `TestUnpagedReads` passes.
6. A budget test pins the unlimited structured search: calls fixed, rows bounded.
7. Dashboard breakdown/table query cards compute over every row (paged).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s, approach follows existing list paging)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Free-text bound: `maxFreeTextSearchResults` (helpers.go) and `freeTextLimit` (queryservice.go).
- List paging: `ListMeta{Total, Page, PerPage, HasMore}` and `listAllEntities` in frontend/src/api/entities.ts (response-driven paging with a runaway guard).
- `listPlan.run` (listpushdown.go) pages a GraphQuery with Limit and Offset.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Rename `maxFreeTextSearchResults` to `maxSearchResults`; it bounds every `/_search` page.
- `queryOptions.offset`; `searchPage` computes size = limit or maxSearchResults.
- `searchPlan.page(ctx, svc, scope, limit, offset)`: limit is always > 0 and always set on the query. With offset > 0, count each type first (one `CountMatched` per type) and skip whole types, set `Offset` on the first type read. Total = sum of counts when counted; otherwise as today.
- Go path: executeQueryWith with limit offset+size+1 (the extra row makes has_more right on free text, whose total is a lower bound), then type filter, total = len, slice [offset:offset+size].
- Handler: parse `page` (1..maxSearchPage), 400 `invalid_page` otherwise; meta Page, PerPage = size, HasMore = offset+len(rows) < total.
- Frontend: `searchEntities` takes `page`; new `searchAllEntities` pages like `listAllEntities`; DashboardView uses it for breakdown/table query cards; SearchView shows a notice when `has_more`.

Alternatives rejected: raising the cap (still unbounded); cursor/keyset paging
(the plan concatenates types and the Go path sorts in Go, so an offset is the
one position both share); computing query-card breakdowns on the server
(TKT-AIEGHU, bigger).

**Files to modify:**
internal/dataentry/{searchplan.go,queryservice.go,helpers.go,entity_read_handler.go,api_v1.go},
tests in internal/dataentry, internal/archguard/unpagedread_allowlist_test.go,
frontend/src/api/entities.ts,
frontend/src/views/{DashboardView.vue,SearchView.vue}, locales, docs-project
GUIDE-acl-security.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `page` query parameter: integer in [1,
maxSearchPage]; anything else is 400 with a constant message. Bounding it keeps
offset*size from overflowing.

**Security-Sensitive Operations:** Read ACL: counts and offsets are computed on
the ACL-scoped queries (`scopedQuery`), so `total`, `has_more` and offsets are
post-gate. No new existence oracle beyond the accepted total.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 handler test with >1000 rows; AC2/AC3 differential test
extended with offsets for both principals and on sqlite/postgres runners that
reuse it; AC4 table test of parseSearchPage; AC5 archguard test; AC6 budget
test; AC7 frontend unit test of searchAllEntities and dashboard card loading.

**Edge Cases:** Offset past the end (empty page, total still reported, has_more
false); offset exactly at a type boundary; an empty type in the middle; page=1
equivalent to no page; free text past the relevance cap returns an empty page.

**Negative Tests:** page=0, -1, abc, page above maximum: 400 invalid_page.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Callers relying on unlimited results: audited (SPA dashboard, search page, bridge `search`, e2e). Dashboard pages; bridge gets the first 1000 with meta, as free text already did.
- Go fallback pages re-read the whole set per page: existing debt tracked by TKT-PSBGQLY.
Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project GUIDE-acl-security.md: the 1000 bound covers every `/_search` query, `page` and `has_more`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-8B4FN7V (significant, addressed), RR-GY2RRI9
(significant, addressed), RR-7MNI7J8 (minor, accepted)
