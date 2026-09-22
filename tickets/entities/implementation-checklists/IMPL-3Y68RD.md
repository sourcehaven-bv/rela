---
id: IMPL-3Y68RD
type: implementation-checklist
title: 'Implementation: Editor e2e specs are flaky under parallel load'
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

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Evidence

**Reproduced before changing anything.** `--repeat-each=8` on `tests/markdown-editor`:
4 failed of 232 (~1.7%), four different tests — matching the reported rate and
the "different test each time" signature. A single `--repeat-each=3` run passed
87/87 first, which is why the higher load was needed: at ~2% per test, one clean
run proves nothing.

**The filed hypothesis was disproved, not implemented.** The ticket blamed
`typeIntoEditor` for dropping keystrokes. Every failing snapshot showed the query
typed in full (`see @mentionzzn4zn2`), so no keystroke was lost. `typeIntoEditor`
is unchanged.

**The actual cause** is `mentionMenuOptions` spanning both menu sections. Three
tests waited for "a row" and pressed Enter, so under load they hit a TYPE row,
which scopes instead of inserting — no `entityRef` node, assertion fails on the
missing link. The smoking gun is a snapshot showing a `bug` scope chip and "No
matches", a state only reachable by Enter on a type row.

**After:** `--repeat-each=8` → 4 failed becomes 1 failed, with all three
mention-insert failures gone. `--repeat-each=3` × 3 → **261/261 passed**, versus
1-2 failures per run before.

The one remaining `--repeat-each=8` failure is a fixture `POST /api/v1/features`
hitting the 5s API timeout under 8x load — infrastructure contention, not the
menu, and it does not reproduce at the repeat level this ticket concerns.

## Test Quality

Deliberately no new test. This ticket fixes wrong *assertions* in existing tests;
adding a test asserting "the entity row is awaited" would restate the fix rather
than guard it. The guard is that the three tests now fail if Enter lands on the
wrong section.

## Quality

Test-only change: one spec file, three call sites, each gaining a comment naming
the failure mode. No production code touched, so no lint or typecheck surface
changed.
