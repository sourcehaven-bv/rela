---
id: IMPL-Y14DKQ
type: implementation-checklist
title: 'Implementation: pin list body-retention and restore export_render row bodies'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (listbodies_test.go: default list, query
shapes, free-text hit-scaling, include_content paying for the page only;
export_list_render_test.go: override receives bodies, built-in table reads none)
- [x] Integration tests written — both suites drive the real handler path
through `storetest.BodyWatch`, which counts bodies served per store call
- [x] Happy path implemented (`loadBodies` seam on the export override path)
- [x] Edge cases from planning handled (denied rows, truncation cap, columnless
list, effective-list fallback, rows walkable twice)
- [x] Error handling in place (`loadBodies` returns an error that propagates
through the export handler rather than degrading to empty bodies)

## Test Quality

- [x] Using table-driven subtests with t.Run
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- The retention defect this bug reports was already fixed by TKT-1U8XYN.
Verified by measurement rather than by reading: a list over a 50-row type reads
**0 bodies**. `store.EntityHeader` carries no body field, so the original
`ListEntities` shape can no longer be expressed at that call site.
- Instrument mutation-verified: forcing `include_content` always-true fails
with `n=50: read 5 bodies, want 0` and `a single-hit query read 2 bodies,
want 1` — on **memstore**, the backend a heap probe reports ~1 MB on either
way. A heap threshold could not separate a 5-body regression from noise.
- Regression mutation-verified: neutering the `loadBodies` call reproduces the
export defect exactly — `row 0 content = "", want "the body of TKT-1"`.
- Full suite: 112 packages, 0 failures. `-race` clean on both touched
packages. `just lint`, `just arch-lint`, `just comment-lint`, `just
plimsoll` green. dataentry coverage 82.8% against a 55% floor.

## Note on the instrument

Acceptance criterion 1 originally specified a DB-gated **heap** assertion,
explicitly not on memstore. It was raised as a deliberate deviation, reviewed,
and the criterion itself was changed to body **counting**
(`storetest.BodyWatch`) — so the ticket now states what shipped rather than
carrying a criterion nobody intends to meet.

The reasoning is on BUG-SDMD6O's test plan and in
[[AM-list-endpoint-bounded-retention]]: counting fails on memstore where a heap
probe cannot, catches a 5-body regression no threshold separates from noise,
and runs under the default `go test ./...` instead of one DB-gated job.
