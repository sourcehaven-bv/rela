---
id: REV-Q1ERIB
type: review-checklist
title: 'Review: App CSP: drop unsafe-inline and split the scaffold into external files'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

**Review Responses:** None created — the two findings that mattered were found
by in-browser verification during implementation and fixed in the same branch
(the editor stylesheet and the e2e fixture; both written up in IMPL-2GEFB2).

The review question that produced them was: *what else injects inline style or
script into an app document at runtime?* That is not answerable from the diff —
the editor bundle is a build artifact, and the e2e app HTML is a TypeScript
string constant. Both were found by loading a real page and by grepping for
inline markup in generated sources.

Repo-wide sweep for anything else now blocked, all clear:

- app HTML files: only `tickets/apps/ticket-counter/` (split here) and the e2e
fixture (split here). `mockups/` and the SPA's own `index.html` are not served
under the app CSP.
- Go sources emitting app HTML: only the scaffold template, which is now
inline-free and has a test to keep it that way.
- docs examples an author would copy: none inline.
- code/tests/e2e depending on `'unsafe-inline'`: none. The remaining mentions
are historical ticket entities (TKT-VEJ39W, TKT-3DBK6I) that correctly record
what was true when written.
- `.style.cssText` in the editor bundle (10 occurrences): CSSOM property
writes, NOT inline style attributes — unaffected by `style-src`. Verified in
browser, not just reasoned about.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All PASS, each verified in a real browser rather than against the header string.
Full evidence tables in IMPL-2GEFB2.

- AC1 scaffolded app works out of the box — PASS (external css/js applied,
`window.rela` present, zero violations)
- AC2 reference app renders identically — PASS (`7px 14px` / `14px` match the
old inline values at the theme's 14px root)
- AC3 `appCSP()` has no `unsafe-inline` — PASS (asserted; mutation reddens it)
- AC4 inline `<script>` and `onerror=` blocked — PASS (both, with a control
proving the OLD CSP lets them run)
- AC5 external `.css`/`.js` serve correctly — PASS
- AC6 docs state the rule — PASS

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
