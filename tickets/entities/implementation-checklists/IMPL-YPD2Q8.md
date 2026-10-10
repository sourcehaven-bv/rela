---
id: IMPL-YPD2Q8
type: implementation-checklist
title: 'Implementation: Implement in-app configuration editing (Configure space)'
started: "2026-10-05"
completed: "2026-10-05"
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race ./...` passes (whole repo). Package tests: `internal/configedit`
(allowlist incl. protective/pinned/merge-key cases, three-way merge, service
save/retry/busy), `cmd/rela-server` (`TestConfigure_*` save, swap under load,
`TestPause_HoldsRecordWrites`), `internal/dataentry` (`TestServeConfigure_Gate`
incl. client and interactive tokens), `internal/appbuild`.
- `e2e/tests/configure.spec.ts` 5/5 against a built `bin/rela-server
--config-editing`: open Configure from the space switcher; rename an entity type
label and see it in `schema.yaml`; rename a property and see the record value
migrated; draft survives reload, then Discard all; edit a board column header
(list item) and see it in `data-entry.yaml`.
- Frontend: typecheck clean, lint 0 errors, `npm run test:run` 3676 tests pass,
build succeeds; `TestNoLabelDerivation` passes.
- Edge cases: refused locked keys (422), stale version (409 conflict), busy
migration lock and write freeze (409/503), migration that never started (409),
roll-forward after a partial migration with Retry, restore of files on a failed
build.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
