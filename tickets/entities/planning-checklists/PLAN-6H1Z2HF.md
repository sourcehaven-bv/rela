---
id: PLAN-6H1Z2HF
type: planning-checklist
title: 'Planning: Search time grows linearly with the data on postgres'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: the postgres search statements (`SearchBackend.Search` and
`Store.SearchVisible`) for a word that most rows contain. Out of scope: the
fs, sqlite and bleve backends (they stay correct and unchanged), and request
time outside the search statement.

**Acceptance Criteria:**

1. A common word reads rows in title order from an index and stops at the
   limit: `TestSearchExplain` checks the plan in the default world and in a
   world with faces, for both builders.
2. A rare word still reads its matches through the search_text trigram index:
   same test.
3. Ranking and visibility behave as before: the store conformance suite
   (`RunVisibleSearchTests`, `RunFieldScopeSearchTests`) and the ranking tests
   in `search_test.go` pass.
4. The scale run's search ratio drops well below 8, and `ratio_max` is
   tightened.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the cause was found with EXPLAIN ANALYZE on the scale-run data; one approach was prototyped and measured)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the fix uses the standard pg_trgm GiST KNN pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- pg_trgm ships a GiST operator class that supports `ORDER BY col <-> text`
  (nearest neighbour), which reads rows in distance order and stops at the
  limit. The GIN operator class cannot order.
- The mention reindexer in pgstore already runs a background pass at open,
  which the title heal pass copies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Store the ranking title as a column (`search_title`), computed by a trigger
from a title map the store records at open, with a GiST trigram index. Order
search by `search_title <-> needle, id`. Resolve faces with an anti-join, so
the planner can keep the index order. Run searches with per-execution plans.

Rejected: a scalar subquery for the world's face (the planner estimates it at
0.5% and never picks the ordered index); computing the title in Go on write
(misses rename, convert and restore statements, and rows written by an older
binary); a GIN index with a sort (still reads every match).

**Files to modify:** `internal/store/pgstore/` search.go, visiblesearch.go,
open.go, pgstore.go, tx.go, new searchtitle.go and migration 0047, tests;
`tools/scalerun/scalerun.yaml`; the scale-run docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The needle is user input and stays a bound parameter. Type and property names
in the title map come from the schema and are bound parameters too.

**Security-Sensitive Operations:**

The visibility clause is unchanged and is applied both to the row and to the
better-face check, so a hidden face cannot shadow a visible one.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria. New tests in
`searchtitle_test.go` cover writes (create, update, rename, type change) and
the heal pass (changed map, removed type, rows left empty by the migration,
more rows than one batch).

**Edge Cases:** types without a title rank by id; an empty title stays empty;
an empty needle orders by id.

**Negative Tests:** an unchanged map queues nothing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The first heal after migrating rewrites every row once (about 0.6 ms per
  row). It runs in batches in the background; until a row is healed it still
  matches but ranks as untitled.
- Mid-frequency words can still pick the bitmap plan when the planner
  underestimates them. That is no slower than before.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: only the scale-run results page changes)

**Documentation Impact:** the postgres scale-run page (numbers and prose).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was prototyped against the scale-run data with EXPLAIN ANALYZE; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
