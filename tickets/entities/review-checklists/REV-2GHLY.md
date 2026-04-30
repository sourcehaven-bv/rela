---
id: REV-2GHLY
type: review-checklist
title: 'Review: Add search interface to data-entry list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` passes
- [x] `just lint` passes (0 issues)
- [x] `just arch-lint` passes
- [x] `just coverage-check` passes (74.2% total)
- [x] `npm run typecheck` passes
- [x] `npm run lint` passes (only pre-existing warnings in unrelated files)
- [x] `npm run test:run` passes (562/562 tests)
- [x] `go test -race ./...` passes

## Code Review

- [x] cranky-code-reviewer agent invoked on diff
- [x] Findings logged as `review-response` entities
- [x] Critical findings addressed (4/4)
- [x] Significant findings addressed (5/8) or deferred with reason (3/8)

12 findings logged total. See REV-2GHLY history for the full table.

**Critical findings (all addressed):**
- RR-W5DGH — Searcher errors silently degrade → addressed (handler returns 500)
- RR-32RJA — Searcher nil-panic → addressed (NewApp validates collaborators)
- RR-NG9Y2 — SearchView regression → addressed (lock only `type`)
- RR-JW7GG — AdHocFilterMenu silent flip → addressed (explicit `mode` prop)

**Significant findings:**
- Addressed: RR-O3LD9, RR-OCKXX, RR-C9TH7, RR-H8X20, RR-C9ZEF
- Deferred with reason: RR-SILQH, RR-I5WU0, RR-YI5PQ

## Acceptance Verification

- [x] Each acceptance criterion from PLAN-XYB07 verified

| AC | Status | Evidence |
|----|--------|----------|
| AC1: SearchBox visible above every list | PASS | Visited `/list/all_tickets`; SearchBox + + Filter visible |
| AC2: Typing filters with debounce | PASS | Vitest integration test asserts exactly one fetch per typed sequence |
| AC3: `q=` hydrates input on mount | PASS | Vitest integration test confirms input value matches URL |
| AC4: Clear-search restores list | PASS | Vitest integration test confirms URL strips q and refetch lacks q |
| AC5: + Filter dropdown lists props | PASS | Browser smoke test: dropdown shows non-FilterBar properties |
| AC6: AND-combines with FilterBar | PASS | Go test `q AND-combines with property filter` passes |
| AC7: Static-pinned hidden | PASS | Browser smoke test: Status/Priority/Assignee absent from menu |
| AC8: `/` focuses, Esc clears | PASS | Puppeteer test: `/` focuses search input; Esc clears + URL |
| AC9: Empty state with clear-search | PASS | Puppeteer test: "No matches for ..." + Clear-search button shown |
| AC10: Backend intersection + sort | PASS | Go test passes (TKT-001 before TKT-003 in list sort) |

## Documentation

- [x] DOCS-92T0E created and marked done

## PR Readiness

- [x] PR creation triggered via `/pr` command (URL recorded after push)
- [x] CI status monitored via `gh pr checks` until green
