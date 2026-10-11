---
id: REV-R1KXVN
type: review-checklist
title: 'Review: Reorder rows of a collection over the incoming side of an orderable relation'
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** `just comment-report` shows no finding in the touched
files that this diff introduces (the one hit, `NeedsRenumber` param-contract in
`internal/entitymanager/order.go`, predates it).

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: run as a worker without subagents; the review was done inline over the full diff and its findings are filed as review responses)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design: RR-PIA9DZ, RR-1PWL4P, RR-Y6O02Y (significant,
addressed); RR-JQ9U0W, RR-ULH7FY (minor, addressed); RR-IIQ4FH (minor,
deferred); RR-2HX7I3 (nit, wont-fix). Code: RR-ZIM04Y (minor, addressed);
RR-BJ9DGD (minor, wont-fix); RR-78W49M (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Incoming list and board tabs show rows in `_order_in` order: PASS
(TestRelationOrder_IncomingTab, e2e "an incoming list tab moves a source"). The
board reads the same page-scoped list and reorders through the same
`useListReorder`, so it has no incoming-specific test of its own.
- Incoming `follow_incoming:` table section is ordered: PASS
(TestRelationOrder_IncomingSection).
- Sort, group and nested display turn the order off: PASS (reader-sort and
author-sort cases; grouping shares the outgoing code path).
- `_order_in` must be readable and writable: PASS
(IncomingHiddenOrderFieldIsNotApplied, IncomingNotMovableWhenFieldReadOnly,
IncomingOrderFieldNotWritable).
- Only visible siblings count, a densify touches only them: PASS
(IncomingStepIgnoresHiddenSiblings, IncomingFacedSources).
- Every written sibling is authorized: PASS
(TestUpdateRelation_IncomingPositionAuthorizesSiblings,
IncomingNotMovableWhenOneSourceDenied).
- Hidden sibling ref gives the uniform 404: PASS
(IncomingHiddenSiblingIsTheUniform404).
- Faced anchors and sources are addressed: PASS (IncomingFacedSources,
TestPlaceOrder_IncomingRefAddresses).
- Size-independent read cost: PASS
(TestQueryBudget_IncomingOrderedListIsSizeIndependent).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EHS7A6

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the task says to commit without pushing or opening a PR)
