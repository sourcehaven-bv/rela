---
id: IMPL-RXQ0PI1
type: implementation-checklist
title: 'Implementation: rela mcp fails to start when flows.yaml names roles'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFlowEngine_InjectedACLChecksFlowsAgainstACLYAML)
- [x] Integration tests written (test full flow, not just units) (the test boots appbuild twice under NopACL, starts a run and compiles its older version)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no acl.yaml: nil policy as before; no flows: acl.yaml not read; declarative injection unchanged)
- [x] Error handling in place (errors surfaced, not swallowed) (an unreadable or schema-mismatched acl.yaml fails the boot, as `rela flow validate` does)

## Test Quality

- [x] Using fixture builders or factories for test data (writeFlowProject, newOnDisk)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by the boot paths in the test and by tickets/ and docs-project/ still starting)

**Verification Evidence:** Before: `rela mcp` on the flowcfg testdata project
exits with `mcp startup: flows.yaml: ... names role "hr", but the project has no
acl.yaml`. After: it answers `initialize`. The regression test fails at boot
without the prepare change and at FlowOf without the engine change.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors newRealtimeConfig, which reloads acl.yaml under an injected ACL)
- [x] Checked for DRY opportunities
- [x] No security issues introduced (the policy is used for config checks only; enforcement is unchanged)
- [x] No silent failures (errors logged AND returned) (exception: loadConfigPolicy tolerates a schema-mismatched acl.yaml (RR-GT629C0) and logs the validation error as a warning instead of returning it (GH-2226))
- [x] No debug code left behind
