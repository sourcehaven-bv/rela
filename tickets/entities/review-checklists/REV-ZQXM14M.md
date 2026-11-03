---
id: REV-ZQXM14M
type: review-checklist
title: 'Review: Property test: two clients autosaving one entity'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend: 365 files, 5480 tests; no Go changes)
- [x] Lint clean (`just lint`) (eslint 0 errors; vue-tsc clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go changes)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage only; no Go changes)

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

**Review Responses:** RR-H4E5X9H, RR-UN9B0H7, RR-LXG3D4T, RR-ZM1SGWM, RR-Q73WATI, RR-WBGKG1P, RR-5OG51BQ, RR-29VGUD0, RR-C5LYNQA, RR-Q3SDLY6, RR-SBZU58L, RR-MX18W6R, RR-S9CWDQT (design review, all addressed). Code review findings are on BUG-SJURXLT.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 no invented values: PASS (checked after every run).
- AC2 no blind overwrite: PASS (checked on every accepted write; the write-ours fault tests fail it).
- AC3 no silent loss: PASS (removing the holds fails it).
- AC4 no false conflict: PASS (found two false conflicts before BUG-SJURXLT's fixes).
- AC5 faults caught: PASS (permanent fault tests; one-off mutations in RR-5OG51BQ and the code review responses).
- AC6 floors: PASS at 300 runs; the property passes 4000 runs.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: test ticket, no user-facing change)
- [x] ~~User-facing documentation updated~~ (N/A: test ticket, no user-facing change; frontend/CLAUDE.md rules updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: test ticket, no user-facing change)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per project owner; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run after done; see note below)

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
