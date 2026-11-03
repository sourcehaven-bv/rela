---
id: IMPL-0D0GXH
type: implementation-checklist
title: 'Implementation: fsstore accepts writes of undeclared types that vanish on reopen'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `internal/storage/dirsegment_test.go` (directory-name rule), `internal/metamodel/plural_test.go` (loader plural and case-insensitive duplicate checks), `internal/store/fsstore/undeclared_type_test.go` (write gate on direct and `Tx` writes, path-like types, empty plural, `SetSchemas` add/remove/refuse/partial apply, soft-delete marks, concurrency)
- [x] Integration tests written (test full flow, not just units): `TestMCPServices_ReloadPicksUpNewType` and `internal/cli/mcp_wiring_fs_test.go` run a real schema hot reload (custom directory, reopen, rejected reload); `TestExportMarkdownData_RefusesUndeclaredType`; watcher tests including `TestExternalStrayCopyLeavesIndexedEntity`
- [x] Happy path implemented: fsstore refuses writes of undeclared types; the MCP schema reload refreshes the store's schemas after publish
- [x] Edge cases from planning handled: empty plural, path-like types and plurals, duplicate plurals ignoring case, removed and re-added types, soft-deleted families, stray same-id files, plural change while entities exist
- [x] Error handling in place (errors surfaced, not swallowed): write errors name the type with `%q`; a refused plural change names the directory rename to do; a refresh failure after publish is logged like other reload errors

## Test Quality

- [x] Using fixture builders or factories for test data: fsstore test schemas come from `storetest.EntityTypes()`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the MCP reload tests run the real reload against an fsstore on disk)
- [x] Each acceptance criterion verified with test scenario from planning: AC1 to AC9 as listed above
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on the default, `postgres`, `sqlite`
and `memorybackend` tags; `go test -race ./internal/...` on the default tag;
sqlite tests on appbuild, cli and store; fsstore with `-race -count=3`;
golangci-lint on all four tags, `just arch-lint`, `just comment-lint`, `just
plimsoll`. Postgres tests were skipped locally (no DSN); CI runs them. Nine
lease and channel CLI tests fail under `memorybackend` on the unchanged base as
well.

## Quality

- [x] Code follows project patterns (check similar code): the refresh is a func the fs recipe supplies at wiring (`Nil: accepted` for database backends); layout publish via `atomic.Pointer`, writers serialized by the existing locks
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one directory-name rule
(`storage.ValidateDirSegment`) shared by fsstore and the metamodel loader; one
storetest type list for test schemas
- [x] No security issues introduced: rela-security-reviewer found one significant finding (plural path escape), addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
