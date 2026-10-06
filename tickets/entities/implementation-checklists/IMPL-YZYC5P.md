---
id: IMPL-YZYC5P
type: implementation-checklist
title: 'Implementation: Boot a SQLite project from its database: schema and acl.yaml through the layered config loader'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation (39b8e316, 72a7c8cb).

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

- 2026-10-04, sqlite builds of `rela` and `rela-server`: a markdown project
was stored with `rela db load --data`, then `schema.yaml`, `entities/`,
`relations/` and `scripts/` were deleted. `rela list doc` returned both
entities. `rela-server -project <dir>` booted on the directory holding only
`.rela/` and served `/api/v1/_schema` (200), the SPA (200) and `/api/v1/docs`
with both entities and their relation.
- `go test -tags sqlite ./internal/appbuild/`: `TestSQLite_BootsFromBakedConfig`,
`TestSQLite_DiskConfigShadowsBaked`, `TestSQLite_BakedACLIsRead`,
`TestSQLite_CollectRefusesSymlinks` pass (2026-10-04). These tests are
sqlite-tagged, so `just test` does not run them.
- `just test`, `just lint`, `just coverage-check` and `just comment-lint` pass
(2026-10-04); they cover the unchanged fs/memory paths.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
