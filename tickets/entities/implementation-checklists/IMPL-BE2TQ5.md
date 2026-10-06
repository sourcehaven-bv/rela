---
id: IMPL-BE2TQ5
type: implementation-checklist
title: 'Implementation: rela db dump / rela db load: export and import a project''s config (and data) to and from rela.db'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation (39b8e316 config half, d988168c `--data`).

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

- 2026-10-04, sqlite build of `rela`: a markdown project (schema, two
entities, one relation, one script) was stored with `rela db load --data`
("Imported 2 entities, 1 relations"; "Stored 2 config files"). The source files
were deleted and `rela list doc` returned both entities. `rela db dump --data
<dir>` wrote them back, and `diff -r` against the original source reported no
differences.
- A later `rela db load --from <dir>` replaced the stored config set, and
`rela-server` booted from it.
- `go test -tags sqlite ./internal/appbuild/`: the `TestImportMarkdownData_*`,
`TestExportMarkdownData_*`, `TestSQLite_Dump*`, `TestSQLite_LoadReplacesTheSet`
and `TestSQLite_Collect*` tests pass (2026-10-04). `./internal/cli/` and
`./internal/config/...` pass with `-tags sqlite`.
- `just test`, `just lint`, `just coverage-check` and `just comment-lint` pass
(2026-10-04).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
