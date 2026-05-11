---
id: IMPL-VYQ99
type: implementation-checklist
title: 'Implementation: Soften workspace write validation per DEC-HWZHA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (IsSoft table test, workspace softening table tests)
- [x] Integration tests written (workspace + dataentry + lua: 7 new test functions)
- [x] Happy path implemented (Layers 0-7 per PLAN-I3A8G)
- [x] Edge cases from planning handled (multiple soft conditions, sorted by path, hard structural stays 422, automation re-validates after property changes)
- [x] Error handling in place (typed errors for hard cases, warnings on result for soft)

## Test Quality

- [x] Using fixture builders (setupSofteningWorkspace seeds canonical task entity)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Each acceptance criterion verified — see workspace softening tests
- [x] Edge cases verified via table tests for IsSoft

**Verification Evidence:**

- `go test ./...` — all packages pass
- `go test -race ./...` — no data races
- `golangci-lint run ./...` — clean (0 issues)
- `just coverage-check` — both thresholds PASS, total coverage 75.3%
- 6 new workspace softening tests cover AC1-5, AC6, AC10
- IsSoft table test (5 cases) pins category classification
- Existing TestCreateEntity_ValidationError flipped to TestCreateEntity_RequiredMissingSurfacesWarning, asserts new behavior

## Quality

- [x] Code follows project patterns (helper next to `newValidationError`, type alias for Warning to keep dataentry call sites clean)
- [x] No security issues introduced (RFC 6901 escaping for property paths via existing utility)
- [x] No silent failures — soft conditions surface as warnings everywhere (HTTP body, MCP tool result with WARNINGS prefix, CLI stderr, Lua second return)
- [x] No debug code left behind
