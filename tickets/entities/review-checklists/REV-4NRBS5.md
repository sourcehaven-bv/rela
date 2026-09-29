---
id: REV-4NRBS5
type: review-checklist
title: 'Review: Frontend builds sub-resource URLs from the route id instead of the served face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend-only change: full vitest suite 212 files / 3338 tests plus faces-backlog e2e)
- [x] Lint clean (`just lint`) (npm run lint: 0 errors; typecheck clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint checks Go; no Go changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors; frontend has no coverage gate)

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

**Review Responses:** RR-BH7D74, RR-ZAR6CB, RR-T4CZOS, RR-4W8DHZ, RR-OD7VF7,
RR-5JFETF, RR-ZCXIPI, RR-RZFHST, RR-X87QW5, RR-43NYRG (addressed); RR-EDTUAH,
RR-EM21EO (wont-fix). Security review: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** Documents PASS, comments PASS, duplicate PASS, relation
picker PASS (four faces-backlog e2e specs flipped from fixme and passing); list
script actions, staged uploads, link-from, export and history PASS (unit tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is created with the prescribed gh command after done)

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
