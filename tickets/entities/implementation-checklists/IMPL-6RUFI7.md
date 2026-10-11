---
id: IMPL-6RUFI7
type: implementation-checklist
title: 'Implementation: Index markdown id mentions as backlinks'
started: "2026-10-07"
completed: "2026-10-07"
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
<!-- Document what you tested and the results -->
- Unit and integration tests:
  - Extraction: `internal/mentions`.
  - Store conformance `RunMentions` on mem, fs, sqlite and pg. Pg was run against a live database.
  - fs: watcher edit, stale cache, cache reuse.
  - pg: an older writer's change is healed; another process's writes are visible.
  - sqlite: an unindexed row is indexed on read.
  - Visibility: world, face, gate, paging and scan budget.
  - REST handler: list, hidden target 404, redacted title, paging, bad input, faced source.
  - CLI table and JSON, MCP (including that a hidden source is not listed), Lua (gated, redacted, paged, bad options), and Vitest for the card.
- e2e: `e2e/tests/mentioned-by.spec.ts`. The card lists and links the source, and is absent with no mentions. 6/6 passed together with `entity-refs.spec.ts`.
- Manual: built `rela` from the branch and ran it on a copy of `tickets/`.
  - `rela show TKT-E9BAAA` listed exactly the 3 files `grep -rl` finds.
  - After appending a mention outside rela, the next run listed 4.
  - A mention inside a fenced code block was not listed.
  - JSON output carried `mentioned_by` and `mentioned_by_truncated=false`.
  - The persisted `mentions-index.json` was reused, and `show` took 0.76s.
- Gates: `just lint` reports 0 issues; arch-lint, plimsoll and the commentlint gate are clean; frontend typecheck and Vitest pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind (the Vitest console.log was removed)
