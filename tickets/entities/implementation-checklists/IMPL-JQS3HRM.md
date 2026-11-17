---
id: IMPL-JQS3HRM
type: implementation-checklist
title: 'Implementation: remote_mcp config fails open on a null allow and on a decode error'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (four TestLoadConfig_RemoteMCP cases)
- [x] ~~Integration tests written~~ (N/A: LoadConfigFromPath is the whole path; planRemoteMCP tests in cmd/rela-server already cover the error propagating to startup)
- [x] Happy path implemented (a duplicate key elsewhere still leaves remote_mcp read)
- [x] Edge cases from planning handled (null allow, commented-out allow, misspelled key next to a duplicate key)
- [x] Error handling in place (errors surfaced, not swallowed) (the spelling check now returns the parse error)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inline YAML strings are the input under test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end (new cases fail against the old config.go and pass with the fix)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Restored origin/develop's config.go: the null-allow,
commented-allow and duplicate-key cases fail. With the fix, internal/project and
the cmd/rela-server remote MCP tests pass.

## Quality

- [x] Code follows project patterns (check similar code) (same node walk as checkKeys)
- [x] Checked for DRY opportunities (the error text matches the empty-list refusal on purpose; two sites, not extracted)
- [x] No security issues introduced (both changes refuse more input)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
