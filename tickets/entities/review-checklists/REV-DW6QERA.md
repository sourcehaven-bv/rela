---
id: REV-DW6QERA
type: review-checklist
title: 'Review: Next-action suggestion grows with the data on postgres'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide tests are left to CI on this machine; ran nextaction, dataentry (targeted, -race), archguard, the pgstore suite with -tags postgres, and TestGoldenParity on default and sqlite)
- [x] Lint clean (golangci-lint serially on the changed packages, default and postgres tags)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: needs the repo-wide test run; CI checks the floors, and the change adds tests to every touched package)

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

**Review Responses:** cranky-code-reviewer: RR-7GVHA0A, RR-C0EH2OA
(significant, addressed); RR-FU1LVY0, RR-O4HKOP7, RR-5E7PT8P (addressed);
RR-J94BJTM (deferred, TKT-J7R3HK6); RR-EMEEQP4 (wont-fix); RR-4Z1A71B,
RR-60T56BI (nits, addressed). rela-security-reviewer: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1-4 PASS (the tests named in PLAN-DDZ5I1Y). 5 PASS:
scale 5 p50 188-201 ms and 34 statements before, 1.9-2.7 ms and 9 after.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VYH8WJA

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh pr create against the stacked base; CI is not monitored for this ticket)

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
