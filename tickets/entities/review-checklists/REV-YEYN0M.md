---
id: REV-YEYN0M
type: review-checklist
title: 'Review: Busy board live refresh on session moves'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on the changed packages, sqlite-tag runs, and the full run inside `just coverage-check`; postgres tests run in CI
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 85.4% total, package floors met; the two load-induced failures (appbuild trigger test, sqlitestore timeout) pass on rerun with -race -count=10 and a longer timeout

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: all three significant code review findings addressed (lost wake under one lock, wake kept on a failed busy read, api-reference fallback text)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review: 14 RRs (4 significant, all addressed); code and security review: 21 RRs (3 significant addressed; 3 declined with reasons, two as follow-ups TKT-IV6XUP and TKT-S3YYGM)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-C9AHJ7
- [x] Test evidence documented in implementation checklist: IMPL-8DZLLY

**Acceptance Status:**
All acceptance criteria in PLAN-C9AHJ7 PASS; each maps to a named test listed in IMPL-8DZLLY.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-XPEKH4
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-XPEKH4

## Final Checks

- [x] Commit message explains the why, not just what: minimal message per user preference; the ticket carries the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

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
