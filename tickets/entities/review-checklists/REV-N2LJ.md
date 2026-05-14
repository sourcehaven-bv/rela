---
id: REV-N2LJ
type: review-checklist
title: 'Review: Migrate workspace from legacy search.Index to bleveindex+search.Service'
status: done
---

## Code Review

- [x] cranky-code-reviewer agent run on the diff
- [x] Critical findings addressed in-PR (#2 Close race, #3 store leak)
- [x] Significant findings addressed in-PR (#4 IndexBatch, #6 doc fix, #11 partial-index visibility)
- [x] Scope decisions documented (#1 phrase semantics — accepted drift)
- [x] Follow-up ticket filed (#7/#13 Observer wiring)
- [x] Tests pass under `-race`
- [x] `just ci` passes end-to-end

**Cranky findings + dispositions:**

| # | Severity | Disposition | Resolution |
|---|----------|-------------|------------|
| 1 | critical | wont-fix (scope) | User chose "just use bleve" — accepted phrase-query drift documented in plan. |
| 2 | critical | addressed | Added `searchWG` WaitGroup; Close waits on subscription goroutine before backend.Close. |
| 3 | critical | addressed | Close uses defer-everything pattern: search Close error captured in `firstErr` but store cleanup still runs. |
| 4 | significant | addressed | Added `bleveindex.Index.IndexBatch([]*entity.Entity) (int, error)` + 2 unit tests. `backfillSearchBackend` uses it. |
| 5 | significant | wont-fix | Same asymmetry exists today; matches `NewForTest`'s "panic loudly" contract. |
| 6 | significant | addressed | Workspace godoc rewritten to describe subscription model accurately. |
| 7 | significant | deferred | Filed as a follow-up ticket: migrate workspace's search subscription to fsstore's `Observers` wiring, eliminating the goroutine entirely. |
| 8–12 | minor/nit | various | #11 (silent partial index) addressed via collected-errors return; #12 nil guards passed (concrete callers already guard); #10 concrete type acceptable per arch-lint allowance. |

**Code Review Summary:**

Pre-PR review caught two genuine bugs (Close race, store leak) and one
performance regression (per-entity backfill) that needed in-PR fixes. All other
findings were scope decisions or follow-up material. Net diff still negative
(~800 LOC delete vs ~140 add including tests).
