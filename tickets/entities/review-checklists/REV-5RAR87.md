---
id: REV-5RAR87
type: review-checklist
title: 'Review: Client baseline drops a role''s face grants when the role also holds "*"'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; only `cmd/rela-desktop` TestChromeStyle_TargetsShippedClasses fails locally because the fresh clone has no built SPA, unrelated)
- [x] Lint clean (`just lint`, plus arch-lint and plimsoll)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (tests added only; floors checked by CI)

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BSYQ33 (minor, addressed), RR-L3MZ4Y (minor,
addressed), RR-LLCLE3 (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (face grants kept for create/update/delete under allowlist and "*" baselines; types outside the ceiling still refused)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: regression tests and Atlas verify.sh (worktree)
pass with the fix and fail without it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, behaviour now matches docs)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

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
