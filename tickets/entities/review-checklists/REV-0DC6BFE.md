---
id: REV-0DC6BFE
type: review-checklist
title: 'Review: Name the page and the app in the browser tab title'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend vitest suite passes (6452 tests before review fixes; the changed suites pass after); no Go change)
- [x] Lint clean (`just lint`) (eslint 0 errors and vue-tsc clean; no Go change)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer over the diff)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (three, all addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-91X00DX, RR-WDV25ZB and RR-4S51RUL (significant,
addressed); RR-8YBETN2, RR-GFTENBO and RR-IBERNSB (minor, addressed);
RR-1C36P5D, RR-B9ABNX7 and RR-O304JE4 (nits, addressed); RR-G2M6BWZ and
RR-NUI5OET (nits, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: the tab reads "<page> · <app name>" on entity, list,
form, view, search, settings and app screens, the app name alone without a page
title, and the (N) count in front (useDocumentTitle.test.ts and the gate tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (data-entry guide, App section)
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after this checklist is done; CI is watched on GitHub)

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
