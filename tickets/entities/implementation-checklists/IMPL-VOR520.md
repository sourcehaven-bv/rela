---
id: IMPL-VOR520
type: implementation-checklist
title: 'Implementation: Data-entry migrations leak a metamodel across rela migrate runs'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (runner_test.go: a metamodel from one run does not reach a later run; existing migration tests set the field directly)
- [x] Integration tests written (test full flow, not just units) (existing internal/projectsetup detect/apply tests cover the full `rela migrate` flow; `go test -race -count=50 -shuffle=on ./internal/projectsetup/` passes)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil metamodel now binds a no-metamodel copy instead of being skipped)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths; WithMetamodel cannot fail)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: one inline YAML document and a mockMetamodel literal, as in the sibling tests)
- [x] No hardcoded values in assertions when object is in scope (the asserted "using metamodel" marker is the migration's own description text; no object holding it is in scope)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-isolation bug; behaviour is pinned by unit and projectsetup tests rather than a manual run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (confirmed the regression test fails on the pre-fix commit and passes with the fix)

**Verification Evidence:**
- `go test -race ./internal/migration/`: pass.
- `go test -race -count=50 -shuffle=on ./internal/projectsetup/`: pass.
- Regression test run against the pre-fix commit 8d77eb1d5: fails with "run without a metamodel reported ... (using metamodel)".

## Quality

- [x] Code follows project patterns (check similar code) (copy-on-bind instead of mutating a shared registry, per CLAUDE.md state rules)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (two one-line WithMetamodel implementations;
nothing to extract)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
