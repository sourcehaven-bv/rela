---
id: IMPL-FP367R
type: implementation-checklist
title: 'Implementation: Review checklist must not track PR URL or CI status — they deadlock the done-before-PR gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no code — a markdown template
and a docs section. The behaviour that matters is enforced by an existing
validation rule, which this change deliberately leaves untouched.)
- [x] ~~Integration tests written~~ (N/A: see Manual Verification — the
integration test is this ticket walking its own workflow.)
- [x] Happy path implemented — the two unknowable items and the `**PR:**`
field are gone; "Run `/pr`" remains
- [x] Edge cases from planning handled — old checklists keep validating; the
no-unchecked rule still bites on the remaining item
- [x] ~~Error handling in place~~ (N/A: no executable code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. **A new checklist reaches `done` with no PR in existence** — this ticket's
own review-checklist is generated from the updated template and closed before
its PR is opened. That is the exact scenario that failed on TKT-MTWQ4G with
`REV-7ILS94: Done review checklists cannot have unchecked items`. Verified by
`rela validate` passing at that point, which is the whole point of the change.

2. **The 171 existing checklists keep validating** — `rela validate --project
tickets --check cardinality --check properties --check validations` → "All
validations passed." The rule reads entity content, not the template, so
historical checklists with the old section are unaffected.

3. **`CLAUDE.md` order matches enforcement** — steps now read Complete
(`done`) → Create PR, matching `/pr`'s gate. Previously the doc said "Create PR
(before `done`)" while the command refused to run until `done`.

**Negative test:** the remaining PR item is a normal unchecked box, so a `done`
checklist that skips it still fails. The change removed the *unknowable* items,
not the enforcement.

## Quality

- [x] Code follows project patterns — mirrors TKT-IVQKQ3, which fixed the same
class of bug (a workflow gate rejecting legitimate shapes) by correcting what
the gate asks for rather than removing the gate
- [x] Checked for DRY opportunities — the ticket→PR link now lives in exactly
one place (git/GitHub) instead of being duplicated into the graph by hand. This
change removes a duplication rather than adding one.
- [x] No security issues introduced — documentation and a markdown template;
no code, no runtime surface
- [x] No silent failures — the validation rule is untouched and still errors
on an unchecked item
- [x] No debug code left behind

**Why a comment replaces the deleted items:** an unexplained omission reads as
an oversight, and the next person adds the items back. The template now carries
the reasoning inline and cites this ticket — the same "write down which of the
two you mean" principle the root `CLAUDE.md` applies to deliberate gaps.
