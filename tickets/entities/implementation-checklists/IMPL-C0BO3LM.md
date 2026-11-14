---
id: IMPL-C0BO3LM
type: implementation-checklist
title: 'Implementation: Record types: pluggable append-only record store for time series and ledgers'
started: "2026-11-14"
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

Headless rela-server on a scratch project with a meter (time series) and an
account (ledger) type, driven with curl against `/api/v1/_records`:

- Append returned 201; list paged with the cursor; hourly aggregate buckets
  summed as expected.
- Missing owner gave 404; wrong owner type gave 400.
- Ledger: balanced posting 201, duplicate group 409, unbalanced 422, posting
  into a closed period 422.
- `_transaction` returned all legs; `_reverse` gave 201, then 409 on a second
  reverse; the balance sum was 0.00.

Automated: `recordstest.RunAll` on file, sqlite and postgres backends;
`TestRecords_GatedThroughPolicy` (owner/reader/outsider, rename, delete) on
default, sqlite and postgres builds; `TestRecordsHandler_Statuses` (17 cases);
retention job tests; full appbuild and dataentry suites pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
