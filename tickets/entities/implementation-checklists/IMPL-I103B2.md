---
id: IMPL-I103B2
type: implementation-checklist
title: 'Implementation: fsstore temp-file cleanup can delete another process''s in-flight write'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `internal/storage/tempfile_test.go` (concurrent writes AC1, mode with umask AC2, failed rename AC4, long and multibyte names, predicates); `internal/analysis/tempfiles_test.go` (AC6); `internal/fsimport/state_test.go` (AC7)
- [x] Integration tests written (test full flow, not just units): `internal/store/fsstore/tempfile_test.go` opens real stores on the OS filesystem: sweep grace period for all temp name forms with real entities and relations kept (AC3), scan ignores temp files (AC5), two stores attaching the same file name at once (AC8), attachment temp sweep; `TestTempFileEventIgnored` for the watcher. AC1 and AC8 fail on develop
- [x] Happy path implemented: unique `O_EXCL` temp names in `SafeFS.WriteFile`; sweeps remove only stale temp files; attachment temp names are random
- [x] Edge cases from planning handled: name-length truncation that keeps UTF-8 intact, legacy `.tmp` and `.new` names still swept, future mtimes treated as fresh, retry on `ErrExist`
- [x] Error handling in place (errors surfaced, not swallowed): every failure after the temp file is created removes it and returns the error

## Test Quality

- [x] Using fixture builders or factories for test data: the store test helpers and `os.Chtimes` for staleness
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the concurrency tests run real writers on the OS filesystem)
- [x] Each acceptance criterion verified with test scenario from planning: AC1 to AC8 as listed above
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on the default, `postgres`, `sqlite` and
`memorybackend` tags; `go vet` and golangci-lint on the changed packages (0
issues); `go test -race` on storage, fsstore, analysis, fsimport, cli, project
and app; `go test ./internal/...`; the concurrency and sweep tests with `-race
-count=10`; `just arch-lint`, `just comment-lint`, `just plimsoll`.

## Quality

- [x] Code follows project patterns (check similar code): the temp-file rule lives once in internal/storage and every sweep and skip calls it
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): the attachment temp prefix, defined in
three places before, is now one constant; `project.AttachmentsDir` replaces a
repeated literal
- [x] No security issues introduced: rela-security-reviewer found one minor finding (attachment leftovers), addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
