---
id: REV-FTKE3U
type: review-checklist
title: 'Review: Drag-and-drop reorder of a list scoped to one entity through an orderable relation'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`: exit 0, race on; frontend vitest 260 files / 3804 tests; rela-components check and 448 tests; e2e relation-order 20/20 over 5 repeats, full suite 440 passed with 1 forms flake that passes alone)
- [x] Lint clean (`just lint`: 0 issues; arch-lint, plimsoll clean; frontend lint 0 errors; typecheck clean)
- [x] Comment lint gate clean (`just comment-lint`; comment-report adds no finding on changed code)
- [x] Coverage maintained (`just coverage-check`: PASS, total 82.8%)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer run on the full diff)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-20SHQW, RR-E9DWET, RR-OOCB04, RR-IP3239)
- [x] Self-reviewed the diff for unrelated changes (only spelling fixes on changed lines; a pre-existing-line change in responses.go was reverted)

**Review Responses:**
- Significant, addressed: RR-20SHQW (moves revealed hidden siblings), RR-E9DWET (densify wrote other faces), RR-OOCB04 (queue stalled after failed refetch), RR-IP3239 (faced anchors could not move).
- Minor, addressed: RR-8WT2EL, RR-FJXDU7, RR-TG3I6W, RR-0FETEE, RR-QNP931.
- Nit, addressed: RR-ZJI841, RR-0SFZQL, RR-ZQ5KR6. Nit, wont-fix with reason: RR-ISU3F6, RR-FB2804.
- Design review responses (23) were addressed during planning.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-2JZXHL)

**Acceptance Status:**
1. PASS: list tab reads in relation order, valueless edges last (relation_order_test.go, api_v1_orderable_test.go).
2. PASS: drag and Alt/arrow moves persist after reload (e2e relation-order.spec.ts list tests).
3. PASS: a column sort hides handles (e2e "sorting by a column removes the handles"; useListReorder inactive test).
4. PASS: a grouped list shows no handles (EntityList active rule; useSectionReorder grouped case).
5. PASS: step across a page edge (TestPlaceOrder step cases; TestUpdateRelation_PositionMoves; planRowMove tests).
6. PASS: board reorder within and across columns (KanbanView.reorder.test.ts; e2e board drag).
7. PASS: section relation order and draggable, not with sort: (relation_order_section_test.go; e2e section move).
8. PASS: movable false and 403 (TestRelationPosition_OrderFieldNotWritable; movable tests).
9. PASS: hidden sibling ref is the uniform 404 (TestRelationPosition_HiddenSiblingIsTheUniform404); a step ignores hidden siblings (TestRelationPosition_StepIgnoresHiddenSiblings).
10. PASS: prev/next walks the tab's order (useScopeNavigation.test.ts).
11. PASS: first move from valueless edges lands where dropped (TestUpdateRelation_PositionDensifiesMissingValues).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (docs/metamodel.md, docs/data-entry.md, docs/data-entry/api-reference.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-T9XEP2

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the user asked for a commit only; /pr runs after done when they want a PR)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
