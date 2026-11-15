---
id: IMPL-YFX6QWO
type: implementation-checklist
title: 'Implementation: Push ingest find and rename dry-run lookups into the store'
started: "2026-11-13"
completed: "2026-11-13"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal query change with no UI; covered by the dataentry webhook tests and golden parity)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Store conformance (`Query/IDTakenFolded`, `SoftDelete`) passes on memstore, fsstore, sqlitestore and postgres.
- `TestHolderLookup_FirstEqual` passes on memstore, sqlite and postgres; `TestHolderLookup_FirstEqualCostIsScaleFree` and `TestRenameDryRun_CostIsScaleFree` pass and fail without the change.
- `TestNoNewUnpagedReads` passes with both allowlist entries removed.
- dataentry `Webhook|Golden|Rename` (-race) and `TestGoldenParity` with `-tags sqlite` pass; no golden changes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
