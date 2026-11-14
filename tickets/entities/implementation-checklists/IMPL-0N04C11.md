---
id: IMPL-0N04C11
type: implementation-checklist
title: 'Implementation: Lower-case sequential id_prefix can never mint a valid id'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths; the change is a comparison)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: a small inline schema is the fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela` for fs, `-tags sqlite` and `-tags postgres` (scratch database
`rela_bug_4opp525`, dropped afterwards) and ran `rela create note` twice on a
schema with `id_prefix: "nt-"`, `id_type: sequential`. Every backend created
NT-001 and NT-002. Before the fix the first create failed with "entity ID
NT-001 does not match any prefix for type note: [nt-]".

Tests: `go test -race` on `internal/metamodel`, `internal/conflict` and
`internal/entitymanager`; `TestValidateCreateIDOpts` and `TestGoldenParity`
in `internal/dataentry` on default and `-tags sqlite`. All pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new error paths)
- [x] No debug code left behind
