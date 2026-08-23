---
id: IMPL-816A2T
type: implementation-checklist
title: 'Implementation: storeutil.TopValues: hoist the triplicated property-value ranking (one copy had already drifted)'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `refactor/storeutil-topvalues-TKT-6QMDLC` · PR #1420.

## Development

- [x] Unit tests written for new code — `storeutil/topvalues_test.go`
- [x] Integration tests written — the three existing conformance suites
exercise the real call sites unchanged; that is the integration coverage for an
extraction, and adding new ones would test the same paths twice
- [x] Happy path implemented
- [x] Edge cases from planning handled — `limit <= 0`, nil/empty map,
allocation sizing, determinism
- [x] Error handling in place — the function cannot fail (pure over a map), so
it returns no error rather than a nil one to ignore

## Test Quality

- [x] Using fixture builders or factories for test data — table-driven with one
shared `counts` map, so a ranking change shows up in every case at once
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — the tie between "open" and "done" at
count 5 exists solely to exercise the alphabetical tiebreak
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — one impl, three delegating sites | **PASS** | `sort` is no longer imported by any of the three files — the signal the cut was at the right seam, not a partial extraction |
| 2 — `limit <= 0` means all; sized to the result | **PASS** | `negative_limit_means_all`; `TestTopValuesAllocatesForTheResult` asserts `cap(got) == len(counts)` |
| 3 — deterministic ranking | **PASS** | `TestTopValuesIsDeterministic` — 50 runs over a 5-way tie |
| 4 — non-nil empty slice | **PASS** | nil and empty map subtests + `require.NotNil` |
| 5 — conformance unchanged | **PASS** | `fsstore` 2.9s · `memstore` 2.5s · `pgstore` 27.9s (live DB) · `storetest` 0.8s · `storeutil` 1.0s; pgstore also green under `-tags postgres` (30.4s) |

**On the drift.** I took pgstore's `len(sorted)` form as the shared version, not
the fs/mem `limit` form, and pinned it with a `cap()` assertion so the bug
cannot be reintroduced by someone "simplifying" the allocation back. Without
that assertion the extraction would have preserved whichever copy I happened to
start from — which is exactly how the drift happened in the first place.

## Quality

- [x] Code follows project patterns — `storeutil`'s package doc states it
exists "to avoid duplicating validation, filtering, and sorted-slice maintenance
logic"; `SortedInsert` / `PaginateSortedKeys` are the precedent
- [x] Checked for DRY opportunities — deliberately extracted the ranking ONLY.
The counting half is genuinely different per backend (prop cache / entity scan /
SQL); a shared version would need a per-backend callback and end up longer than
the duplication it removed
- [x] No security issues introduced — pure function over an already-validated
map; no new input, no I/O
- [x] No silent failures — the function cannot fail; the one contract that
could silently break a caller (nil vs empty slice on the wire) is tested
- [x] No debug code left behind
