---
id: IMPL-3MJLR1L
type: implementation-checklist
title: 'Implementation: rela validate accepts deprecated link: true that rela-server refuses'
started: "2026-10-30"
completed: "2026-10-30"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: small inline YAML fixtures, as in the neighbouring tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Built `rela` from the branch. In a fresh project with a list column `link: true`, `rela validate` printed "data-entry.yaml uses deprecated syntax: - Convert link: true to link: detail ... Run 'rela migrate'" and exited 1. After `rela migrate` the column read `link: detail` and `rela validate` exited 0.
- `TestValidateWithFS_DeprecatedDataEntry` (list column and view section `link: true` fail with a migration error and `HasErrors()`; `link: detail` is valid) fails on develop and passes on the branch.
- `TestConfigure_DeprecatedDataEntry`: the Configure preview reports exactly one problem, `data-entry.yaml: deprecated syntax: Convert link: true to link: detail in data-entry.yaml` (no `rela migrate` hint: the preview edits unsaved bytes).
- `just docs` regenerated docs/data-entry.md; `grep -c 'link: true'` is 0 in both the guide and docs/.
- go test for projectsetup, migration, cmd/rela-server and internal/cli: ok. golangci-lint on the touched packages: 0 issues. `just arch-lint` and `just comment-lint`: clean. `just plimsoll` could not load packages under machine load (WaitDelay); the change adds one method to `migration.Error`, which has two.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes:** cmd/rela-server may not import internal/migration (arch-lint), so
configure.go matches the error through a call-site interface `interface{
Descriptions() []string }`, and `migration.Error` gained `Descriptions()`.
