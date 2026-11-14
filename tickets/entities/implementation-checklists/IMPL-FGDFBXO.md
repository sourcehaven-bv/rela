---
id: IMPL-FGDFBXO
type: implementation-checklist
title: 'Implementation: loadConfigPolicy drops the acl.yaml validation error silently'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is one log call inside a boot path; covered by the boot subtest below)
- [x] Integration tests written (test full flow, not just units) (subtest "acl.yaml that does not fit the schema still boots" boots appbuild under NopACL and asserts the warning)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil meta and missing acl.yaml skip validation as before; a load error is still returned)
- [x] Error handling in place (errors surfaced, not swallowed) (the validation error is now logged as a warning)

## Test Quality

- [x] Using fixture builders or factories for test data (writeFlowProject, newOnDisk, captureWarnings)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the boot subtest exercises the full appbuild path that rela mcp uses)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: unchanged edges are covered by the existing subtests)

**Verification Evidence:** The new assertion fails on origin/develop's
realtime.go and passes with the fix; `go test ./internal/appbuild/` passes.

## Quality

- [x] Code follows project patterns (check similar code) (slog.Warn for a tolerated startup problem, as TKT-T31NKT and TKT-DOFYR1)
- [x] Checked for DRY opportunities (reuses captureWarnings)
- [x] No security issues introduced (log only; the policy and enforcement are unchanged)
- [x] No silent failures (errors logged AND returned) (exception: the validation error is logged but not returned, because a host that does not enforce acl.yaml must still boot (RR-GT629C0))
- [x] No debug code left behind
