---
id: IMPL-ZMDIN2X
type: implementation-checklist
title: 'Implementation: Lua rela.attach_file: store script bytes on a file property'
started: "2026-11-15"
completed: "2026-11-15"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: the dataentry tests run real action scripts through the HTTP action handler, ACL and store; no separate rela-server run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `TestAttachFile_ActionStoresBytes`: binary PDF bytes with NUL and 0xFF are stored unchanged; hash matches the API listing.
- `TestAttachFile_Refusals`: no update grant, hidden entity, missing entity, non-file property, accept: rejection, sniff contradiction, wrong extension, malformed mime. Nothing stored in any case.
- `TestAttachFile_SizeLimitApplies`, `TestAttachFile_RejectedFileIsAudited`.
- `TestAttachFile_OutsideRequestIsGated`: schedule/flow-style bound ctx; unbound ctx refused.
- `TestAttachFile_*` in internal/lua: argument misuse raises, refusal returns nil+err, absent without filer, refused in a store transaction.
- `TestAttachFileHolder`, `TestScheduledLuaWriteDeps_CarriesAttachFile`: schedules and flow steps get the binding.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
