---
id: IMPL-SWNQG6
type: implementation-checklist
title: 'Implementation: rela acl who-can <verb> <entity> — list principals with access to one entity + provenance (UC3)'
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

**Verification Evidence:** Built `rela` and ran against a scratch ISMS project
(person/team/folder/incident + acl.yaml). `who-can read INC-042` returned the
four expected readers each with its route (global / group / local /
local-via-ancestor); `who-can delete INC-042` correctly narrowed to the
write-granted set; `--output json` emitted the versioned list-of-routes shape; a
missing entity exited non-zero. Automated: `internal/aclmap` read-vs-runtime
conformance test asserts `who-can read E == {p : PermitsRead(E)}` — caught a
real group-member enumeration bug during development, then passed after fix.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — grant-verb mapping centralized in `grantForRole`; provenance rendering shared; no premature abstraction
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
