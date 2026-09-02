---
id: IMPL-FGVU3N
type: implementation-checklist
title: 'Implementation: Gantt perf: header projection + SQL-scoped subtree drill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`loadGanttType` (verdict switch: headers / fallback / deny),
`buildGanttSubtree` (GraphQuery closure fast path), `finishGanttForest`
extraction shared by both paths. New tests:
`TestGantt_SubtreeDrillMatchesFullBuild` (byte-equality with the full build,
fold through the SQL closure, breach survival),
`TestGantt_SubtreeDrillWhereExcludedRoot`. All 20 prior gantt tests green on
the new code paths (ACL tests exercise the fallbacks).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Equivalence test compares the drilled response against the SAME subtree cut
from the full response — the full build is the oracle, not literals.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** re-ran the postgres harness
(`.ignored/gantt-perf/`, real `rela-server-postgres`, 20k/50k schemas):
full 387→219 ms (1.8×), drill 327→55 ms (6×) at 50k; drilled payload
inspected (66-node subtree, correct rolled span and breach). Full table in
REPORT.md.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Fast paths verdict-gated with honest fallbacks; redact-exactly-once preserved
per path; shared tail prevents semantic drift. Gates: golangci-lint 0,
arch-lint (with a documented `.ignored/` exclude for gitignored scratch),
comment-lint gate clean, full dataentry suite green.
