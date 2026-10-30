---
id: REV-3XNDZCN
type: review-checklist
title: 'Review: Point to RELA_DATAENTRY_USER on acl_unstamped_principal'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test ./internal/dataentry/... ./cmd/rela-server/... ok; the change is confined to those packages)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages: 0 issues; `just arch-lint` clean. Repo-wide runs fail to load packages under load from parallel sessions; CI runs them)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (cmd/rela* is excluded from the coverage gate in .testcoverage.yml; the new predicates still have table tests. internal/dataentry changes only a string literal; CI runs the gate)

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

**Review Responses:** Design: RR-Z2RB0BE, RR-VEV5702 (addressed). Code:
RR-3MALUVU (significant, addressed), RR-KD05YX5 (security, minor), RR-YL7PK19,
RR-VCP4MYJ, RR-VP45T87, RR-DB2QKKA, RR-C3LZVOD, RR-IHD0DQ0 (all addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS, adapted to BUG-C8DHGF: the no-user case is now
401 `unauthenticated`, and its detail carries the hint
(TestACLMiddleware_AnonymousIsUnauthenticated; golden anonymous.golden updated).
AC2 PASS (TestWarnUnidentifiedACL warning rows). AC3 PASS (no-warning rows,
incl. JWT and env user). AC4 PASS (manual rela-server run: warning once at
startup, curl shows the hint; with RELA_DATAENTRY_USER no warning).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-TQC1GXX

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
