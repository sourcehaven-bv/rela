---
id: IMPL-I5O665
type: implementation-checklist
title: 'Implementation: Split metamodel.Metamodel into focused types'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestNewValidator_RejectsNilMetamodel)
- [x] ~~Integration tests written~~ (N/A: no new behaviour; the existing entitymanager, store, migration and CLI suites exercise every moved call)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests rewritten call-for-call; the only new fixture is testSchemaMetamodel in cli)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no new interpolated assertions)
- [x] ~~Property comparisons use original object~~ (N/A: no new property comparisons)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal refactor with no behaviour change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
1. `go test` on all 16 touched packages passes, default and sqlite tags; every package compiles its tests under default, memorybackend, postgres and sqlite.
2. `git diff` of projection.go, shapeprojection.go and validation.go: only signatures, receivers (`m.` to `v.meta.`) and doc lines change.
3. `just plimsoll` passes with `max-exported-methods=21`.
4. `just arch-lint` OK; `golangci-lint run ./...` 0 issues; `just comment-lint` clean.
Call sites were rewritten from compiler errors, not text search, so
`datamigration.State.ShapeProjection` and other same-named methods were left
alone.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (none: one-line call-site rewrites)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
