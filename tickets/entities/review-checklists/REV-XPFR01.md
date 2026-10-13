---
id: REV-XPFR01
type: review-checklist
title: 'Review: Create menu offers only linkable types (TKT-XPFXMG)'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend `npx vitest run`: 291 files, 4146 tests passed; e2e space-create-page-link, spaces, pages and relation-order specs: 21 passed against a fresh `just build-server-e2e`; no Go changes)
- [x] Lint clean (`just lint`) (eslint 0 errors, only warnings that predate this branch; `vue-tsc -b` and e2e `tsc --noEmit` clean; no Go changes for golangci-lint)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (own review of `git diff origin/develop...HEAD`, then the cranky-code-reviewer agent on the branch plus the first fix)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-XPFB02, RR-XPFC03, RR-XPFD04, RR-XPFE05)
- [x] Self-reviewed the diff for unrelated changes (diff limited to the Create menu, the pages store, PageView, their tests, the e2e spec, the data-entry guide and the ticket entities)

**Review Responses:** RR-XPFA01 (two-relation type hidden; wont-fix, by
acceptance criterion, docs now say how to create it), RR-XPFB02 (full menu
while the anchor loads; fixed with `pageStore.opening`), RR-XPFC03, RR-XPFD04,
RR-XPFE05, RR-XPFF06, RR-XPFG07 (busy button instead of hiding it; keyboard
cannot open an empty menu), RR-XPFH08 (wont-fix), RR-XPFI09, RR-XPFJ10.

## Acceptance Verification

- [x] Each acceptance criterion tested (criteria are on TKT-XPFXMG; no planning checklist was made for this ticket)
- [x] Test evidence documented in implementation checklist (no implementation checklist exists for this ticket; the evidence is listed below)

**Acceptance Status:**
1. PASS: on an entity page only linkable types are offered, per tab
   (SpaceCreateMenu.test.ts 'offers only the types it can link', 'offers what
   the open tab can link'; e2e Tasks tab offers Task and Bug, Blockers tab
   reached by a tab click offers Bug).
2. PASS: off an entity page every type is offered (unit 'offers every type
   outside an entity page'; e2e spaces.spec 'the Create menu offers the
   space's types').
3. PASS: with no linkable type the button is hidden (unit 'hides the button
   on an entity page that can link none of the types').
4. PASS: an open dialog survives a navigation that empties the menu (unit
   'keeps an open dialog when a navigation empties the menu').
5. PASS: docs/data-entry.md § Entity pages describes the behaviour, including
   the busy button until the entity has loaded.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-data-entry.md, regenerated into docs/data-entry.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-R6A3E4

## Final Checks

- [x] Commit message explains the why, not just what (the first commit names the behaviour; the review commit is minimal by the owner's convention and the why is in the review responses)
- [x] No TODOs or FIXMEs left unaddressed (none in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1876 exists; CI is monitored after push)

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
