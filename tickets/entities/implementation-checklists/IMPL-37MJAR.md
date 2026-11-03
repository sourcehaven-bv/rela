---
id: IMPL-37MJAR
type: implementation-checklist
title: 'Implementation: Prose-check registry and engine with analyze prose'
started: "2026-10-30"
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
A scratch project (`.ignored/prose-e2e`) with the 46 real ISMS procedures and
10 tasks, a `procedure` and a `task` type, and the prototype's Vale styles in a
config outside the project (`[*.md]` base, `[**/procedure/*.md]` adds
ISMS-Overspecifiek):

- `rela analyze prose`: 596 findings in 53 entities in 3.3 s, exit 0. The
  procedure-only style produced 115 findings, all on procedures and none on
  tasks (AC2).
- The first run left one finding without a position: Vale reports a match
  across a line break (`wordt gebruik\ngemaakt`) as one span. Fixed: a span
  may continue past the line end; a test pins it. After the fix every finding
  has a position.
- `rela -o json analyze prose --type task`: status warning, 15 findings in 7
  entities, with `match` taken from the body, including `geüpload` (AC3, AC5).
- `rela analyze all`: no prose output. `rela analyze all --prose`: summary
  shows `Prose: 596` and a Prose Checks section (AC5).
- A checker without `--no-exit` and a missing config: each entity lists
  `checker "isms": exit status 2` with Vale's message; the command fails with
  "10 checker run(s) failed" (AC4).
- A type named `my task` with checkers declared: the load fails naming the
  type and the allowlist (AC1).

Automated: `go test` on cmdexec, check, metamodel, cli, configedit, dataentry
and rela-server; golangci-lint and go-arch-lint clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
