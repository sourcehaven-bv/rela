---
id: IMPL-ZG9HNOX
type: implementation-checklist
title: 'Implementation: List toggle widget'
started: "2026-11-24"
completed: "2026-11-24"
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

**Verification Evidence:** Ran rela-server on a copy of
prototypes/data-entry/project with `status` as a toggle (`on: resolved, off:
in-progress, other: disabled`). The list showed resolved rows checked, open and
closed rows locked. Checking an in-progress row wrote `resolved` to the file;
unchecking wrote `in-progress`. With `off: open` every box locked, because the
machine declares neither open→resolved nor resolved→open; a direct PATCH of
open→resolved returns the server's "illegal transition" error.
`?field_verdicts=status` returned `_fields` and `_redacted` per row.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
