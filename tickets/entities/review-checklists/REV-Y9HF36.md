---
id: REV-Y9HF36
type: review-checklist
title: 'Review: Create a linked entity from a markdown task item'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (every changed package passes; the last local run timed out only in sqlitestore under machine load, which this branch does not touch; CI reruns it)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** two rounds, 65 linked responses. All critical and
significant ones are addressed; one minor is wont-fix and one nit deferred, each
with a reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: offers are absent for a type with no recipe (TestTaskItemOffers; editor tests).
2. PASS: link and remove modes, relation direction, autosave (e2e task-item-conversions.spec.ts; taskItemConvert tests).
3. PASS: several recipes open a menu (TaskItemConversions tests).
4. PASS: no button on create forms, plain bullets, empty or reference-only items (editor and DynamicForm tests).
5. PASS: required-fields dialog, cancel creates nothing (TaskItemConversions tests, including Escape during preflight).
6. PASS: create, relation and update rights; 403 and uniform 404 (entitymanager and handler tests).
7. PASS: invalid config fails the load with specific errors (TestTaskItemConversionValidation).
8. PASS: faced target created at its face (TestTaskItemConversionFacedTarget and converter tests).
9. PASS: audit and on-create automations run (converter tests).
10. PASS: item text is never expanded (converter tests).
11. PASS: item-text fields pass the field write gate (TestTaskItemConvert_GatesFieldsTheItemTextFills).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-URK57C

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the commit message is kept minimal by the user's standing instruction; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after the ticket is done; see the note below)

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
