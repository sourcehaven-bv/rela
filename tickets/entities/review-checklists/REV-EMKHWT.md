---
id: REV-EMKHWT
type: review-checklist
title: 'Review: Create menu puts the current page''s type first'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend `npx vitest run`: 305 files, 4294 tests passed after review fixes; e2e spaces, space-create-page-link and pages specs pass against a fresh `just build-server-e2e`; no Go changes)
- [x] Lint clean (`just lint`) (`vue-tsc -b` clean, eslint adds no warnings, e2e eslint and `tsc --noEmit` clean; `just ci` run, see PR)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go changes; the frontend has no coverage gate)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer on the first commit)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-VX2G41: open-tab rule shared through `routeTabId`)
- [x] Self-reviewed the diff for unrelated changes (prettier reflows of untouched lines reverted)

**Review Responses:** RR-VX2G41 (significant, addressed), RR-RA7N5P,
RR-PK8I7V, RR-ZD0XO6, RR-BPIEMG, RR-7DO2HI (minor, addressed), RR-W7BKPS
(minor, wont-fix: search and dashboard have no Create menu), RR-54FN6K,
RR-13RBT1, RR-98SI6C (nit, addressed), RR-B6I3OX (nit, wont-fix: dupes check
ignores tests).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (PLAN-1JKO7Y)
- [x] Test evidence documented in implementation checklist (IMPL-DBA678)

**Acceptance Status:**
1. PASS: list page leads with its type (unit "puts a list's type first"; e2e list/bugs and list/tasks).
2. PASS: kanban page leads with its type (unit "puts a board's type first"; e2e kanban/bug-board).
3. PASS: calendar with several types leads with them in config order (unit "leads with a calendar's types in config order"; gantt in useViewEntityTypes.test.ts).
4. PASS: page tab decides (unit "follows the view of the open page tab", entity page case, pageTabView tests).
5. PASS: config order elsewhere (unit "keeps config order on a page without view types"; e2e list/features).
6. PASS: separator only when rows follow (unit "adds no separator when nothing follows the leading type"; e2e separator counts).
7. PASS: docs/data-entry.md § Spaces describes the order.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-data-entry.md, regenerated into docs/data-entry.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DSV001

## Final Checks

- [x] Commit message explains the why, not just what (minimal one-line messages by the owner's convention; the why is in the ticket)
- [x] No TODOs or FIXMEs left unaddressed (none in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; CI is monitored after push)

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
