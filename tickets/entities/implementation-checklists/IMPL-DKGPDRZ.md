---
id: IMPL-DKGPDRZ
type: implementation-checklist
title: 'Implementation: Gantt read grows faster than the data on postgres'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Perf project on local postgres, `rela-server-postgres`, gantt `delivery` root
view. p50/p95 before: about 220 ms / - at scale 1 and 2.7 s / - at scale 5.
After: alice 167/188 ms and carol 164/182 ms at scale 1; alice 879/1276 ms and
carol 822/1043 ms at scale 5. Statements at scale 5: 229 (CI baseline) to 119.
- `TestListRelations_LargeIDBatchPages` and
`TestListRelations_LargeIDBatchIsNotPrepared` fail with one-shot disabled (3
statements instead of 1; 2 prepared statements left behind) and pass with it.
- `TestGanttEdges_OneStatementPerRelationType_Postgres` holds one statement at
100 and 1,200 edges.
- Full pgstore suite with `-race -tags postgres`, gantt and `TestGoldenParity`
on default and `-tags sqlite`: pass, no golden change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
