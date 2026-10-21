---
id: IMPL-5N181W
type: implementation-checklist
title: 'Implementation: fsstore index cache can drop an entity written by a concurrent process'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `internal/store/fsstore/index_scan_test.go` with `TestIndex_ConcurrentWriterIsSeenOnReopen` (AC1), `TestIndex_SchemaChangeIsSeenOnReopen` (AC2, type added and type removed), `TestIndex_NoPersistedIndex` (AC3) and `BenchmarkOpen` (AC5)
- [x] Integration tests written (test full flow, not just units): AC1 uses two real fsstore instances on one temp directory with the real FS, as two CLI processes would; all three tests fail on develop
- [x] Happy path implemented: every open scans the entity and relation directories; `Close` writes no index
- [x] Edge cases from planning handled: pending deletes still hide marked families after reopen (existing soft-delete tests), a planted stale index file has no effect, schema add and remove
- [x] Error handling in place (errors surfaced, not swallowed): the scan's error handling is unchanged; one fewer silently ignored write (`_ = s.savePersistedIndex()` is gone)

## Test Quality

- [x] Using fixture builders or factories for test data: the package's test schemas and store helpers
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: AC1 reproduces the reported two-process scenario with real stores on a real directory)
- [x] Each acceptance criterion verified with test scenario from planning: AC1 to AC3 by the tests above, AC4 by the existing pending-deletes tests, AC5 by `BenchmarkOpen`
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on the default,
`postgres`, `sqlite` and `memorybackend` tags; `go test -race` on
internal/store/..., internal/app/..., internal/appbuild/..., internal/fsimport,
internal/cli/... and cmd/rela-server/...; `just arch-lint`, `just comment-lint`,
`just plimsoll`. Coverage: fsstore 84.5%, app 76.2%, appbuild 73.9%, fsimport
74.7%. `BenchmarkOpen` (3,000 entity and 2,000 relation files): about 16 to 20
ms per open, against about 103 ms per open and close on develop.

## Quality

- [x] Code follows project patterns (check similar code): removes code; the scan path was already the cold-open path
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): two restore paths and the save path
removed; one construction path remains
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
