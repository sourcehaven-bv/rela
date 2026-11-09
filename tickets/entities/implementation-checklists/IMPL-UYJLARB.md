---
id: IMPL-UYJLARB
type: implementation-checklist
title: 'Implementation: Remote pull: refuse symlinked credential files and hide unreadable conflict rows'
started: "2026-11-09"
completed: "2026-11-09"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: a pull needs a second rela; the full run is covered by remotesync tests over a fake source and a real ACL policy)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** TestCredential_ReadDeclaredSymlink,
TestCredential_ReadDeclaredHardLink, TestRun_ConflictHidesUnreadableLocalRow
(real acl.yaml policy), TestRun_HiddenLocalRowsAreNotNamed (update, relation
endpoint, gone entity, gone relation). Each hidden-row test fails with the gate
check disabled.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: `readable`, `gateFailed` and `endpointsReadable` share the gate check
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
