---
id: REV-LR717X
type: review-checklist
title: 'Review: Session default_in: offer keyed types only where they apply'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on the changed packages, sqlite-tag runs, and the full run inside `just coverage-check`; postgres tests run in CI
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 85.4% total, package floors met

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
- [x] All significant review-responses addressed: the one significant finding (reload test could not fail) is addressed with TestReload_DefaultIn
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** design review: 12 RRs (2 significant addressed, 1 minor wont-fix); code and security review: 13 RRs (1 significant addressed, 1 minor wont-fix with reason)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-3JOPEX
- [x] Test evidence documented in implementation checklist: IMPL-ZCJCLB

**Acceptance Status:**
All acceptance criteria in PLAN-3JOPEX PASS; each maps to a named test listed in IMPL-ZCJCLB.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-ZSQPFQ
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZSQPFQ

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
