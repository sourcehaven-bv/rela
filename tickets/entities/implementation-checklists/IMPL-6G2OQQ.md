---
id: IMPL-6G2OQQ
type: implementation-checklist
title: 'Implementation: Data-entry, templates and scripts read config through the services'' config loader'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation (44e2e0d8, 6a6e8b42, a9b571c2).

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

- 2026-10-04, sqlite build of `rela-server` on a directory holding only
`.rela/`, with `schema.yaml`, `data-entry.yaml`, `scripts/` and
`custom/custom.css` stored by `rela db load`: the server read `data-entry.yaml`
from the database and booted, served the SPA (200), the entity API, and
`/_custom/custom.css` with the stored content (200).
- Not exercised by hand: actions, Lua validations, scheduled tasks and
templates against a database-only project. They share the `ReadScript` and
loader seams pinned by `TestSQLite_BakedScriptsAreRead` and
`TestSQLite_BakedTemplatesAreUsed`.
- `go test -tags sqlite ./internal/appbuild/`: `TestSQLite_BakedScriptsAreRead`,
`TestSQLite_BakedAssetsAreServed`, `TestSQLite_BakedTemplatesAreUsed` pass
(2026-10-04). `go test` of `internal/dataentry`, `internal/script`,
`internal/lua`, `internal/validation`, `internal/mcp`, and of `internal/rootfs`
and `internal/config/...` with `-tags sqlite`, passes.
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

The duplicated `os.OpenRoot` readers in `script/action.go`,
`script/executor.go`, `validation/lua.go` and `dataentry/custom.go` were
replaced by `lua.ReadDeps.ReadScript` and `internal/rootfs`.
