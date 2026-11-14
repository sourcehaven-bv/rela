---
id: IMPL-0OLPPFA
type: implementation-checklist
title: 'Implementation: Archguard allowlist for unpaged list reads'
started: "2026-11-13"
completed: "2026-11-13"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: matcher cases are inline Go source snippets)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test -count=1 ./internal/archguard/` passes. The
first draft's real-tree run listed new helper callers and stale entries, which
confirmed both the growth and the stale-entry failures end to end.
`TestScanUnpagedReads_FindsSyntheticRead` plants a read in a temp tree.
`golangci-lint` on the package, `just comment-lint` and `just arch-lint` are
clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — reuses scanTree, diffAllowlist, isEmptyString and mkdirAllWrite from the existing guards
- [x] ~~No security issues introduced~~ (N/A: test-only code)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
