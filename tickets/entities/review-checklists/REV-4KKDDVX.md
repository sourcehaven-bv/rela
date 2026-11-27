---
id: REV-4KKDDVX
type: review-checklist
title: 'Review: TestSSEACL tests flake on a fixed settle() sleep'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (ran narrowly on an overloaded machine: TestSSEACL -race -count=50 and -count=20, plus TestSSE|TestLive|TestWatcher once; full suite deferred to CI)
- [x] ~~Lint clean (`just lint`)~~ (deferred to CI: local runner overloaded; gofmt and go vet clean)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (deferred to CI: local runner overloaded)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-only change; no production code changed)

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
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->
RR-MNX2LB9, RR-0BSL44I, RR-0R2WEEJ (addressed); RR-P44L7NU, RR-673T9IH, RR-TBEZQ45 (wont-fix with reason)

## Acceptance Verification

- [x] Each acceptance criterion tested (no fixed sleeps remain; negative checks proven by a leaking-gate mutation)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->
PASS: settle() and the 50 ms sleep are gone; 50 stress runs passed; a leaking gate fails all six negative checks.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per user preference; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (PR opened with gh pr create after the ticket is done; the user handles merge)

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
