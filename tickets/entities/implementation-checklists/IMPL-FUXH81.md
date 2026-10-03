---
id: IMPL-FUXH81
type: implementation-checklist
title: 'Implementation: Path-scoped agent rules instead of one large CLAUDE.md'
started: "2026-10-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the guard test runs against the real repository, which is the full flow)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: table-driven glob cases; no domain objects)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Line check: a script compared every non-blank line of the old root
  `CLAUDE.md` with the new root plus the rule files; 0 lines missing.
- `go test ./tools/agentrules/` passes; changing `internal/mailrender/**` to a
  misspelled path made `TestRulePathsMatchFiles/mail.md` fail with
  "matches no file".
- `git status` showed `.claude/rules/*.md` as new files while the tracked
  `.claude/agents` and `.claude/commands` files stayed unchanged.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
