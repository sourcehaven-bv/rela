---
id: IMPL-VYWMQ8
type: implementation-checklist
title: 'Implementation: export: route entity/list export + export_render through visibility.Reader; thread request principal into ExecuteDocument (closes #1188 IB finding)'
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

- [x] Feature manually tested end-to-end — via full HTTP-layer tests (httptest → handler → real memstore + acl.Declarative + affordances resolver → real `cp` transform → response bytes asserted)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- New tests (all pass first run): TestExport_Entity_HiddenFieldRedacted (AC1), TestExport_Entity_HiddenTitleFallsBackToID (AC1), TestExport_Entity_VisibleNeighborHiddenTitle (AC3), TestExport_List_FieldVisibility {HiddenColumnEmptyAndNeighborID, HiddenTitleColumnFallsBackToID} (AC2+AC3), TestExecuteDocument_PrincipalThreaded (AC4).
- All 13 pre-existing export tests green unchanged (AC5 parity + AC6 denied-404 pins).
- Full sweep: `go test` over every package except docscapture (pre-existing env failure) — green. golangci-lint 0 issues; arch-lint OK (visibility added to dataentry.mayDependOn); plimsoll OK; gofmt clean.
- Changes: exportHandler reads through visibility.Reader (Get owns the stored-type check; caller check removed), list rows through Filter, neighbor titles through new exported visibility.Redact; ExecuteDocument(ctx) threads WithContext+WithPrincipal mirroring execute(); docs/transforms.md gains an Access-control section incl. the documented PR-3 residual.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (Redact hoisted into visibility rather than re-implementing the copy-strip in dataentry)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
