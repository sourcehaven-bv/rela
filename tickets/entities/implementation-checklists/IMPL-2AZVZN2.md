---
id: IMPL-2AZVZN2
type: implementation-checklist
title: 'Implementation: Flow subject or create type given as an alias skips the visible: check'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (two `TestParse_Problems` cases, `TestParse_AliasSubjectReadsNoHiddenProperty`)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: config-load check; `Parse` is the full path and is what the tests call)
- [x] Happy path implemented
- [x] Edge cases from planning handled (alias in `subject` and in `create.type`)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (`parseSample` edits the shared sample)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: assertions match fixed config keys and messages)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end (new tests fail with the fix removed and pass with it)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Removed the fix: `TestParse_Problems` and
`TestParse_AliasSubjectReadsNoHiddenProperty` fail. With the fix: `go test -race
./internal/flowcfg/ ./internal/appbuild/` and `./internal/cli -run Flow` pass;
`just arch-lint` and golangci-lint on `internal/flowcfg` clean.

## Quality

- [x] Code follows project patterns (check similar code) (message matches `internal/realtimecfg/validate.go`)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
