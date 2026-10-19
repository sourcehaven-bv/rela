---
id: REV-MHEAV4
type: review-checklist
title: 'Review: Scheduled security scan: gitleaks, semgrep, osv-scanner'
started: "2026-10-18"
status: done
completed: "2026-10-18"
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: workflow and config only, no Go or frontend code)
- [x] ~~Lint clean (`just lint`)~~ (N/A: workflow and config only, no Go or frontend code)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: workflow and config only, no Go or frontend code)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: workflow and config only, no Go or frontend code)

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

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: workflow-only change; checked with actionlint and zizmor)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `on:` has schedule and `workflow_dispatch` only.
2. PASS: alerts under gitleaks, semgrep and osv-scanner after run 39113141877.
3. PASS: #1922 created, then closed by run 39113412130; #1923/#1924 edited.
4. PASS: gitleaks 0 locally and in run 39113412130.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR #1925 opened with gh before this ticket existed)

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
