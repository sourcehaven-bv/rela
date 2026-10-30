---
id: REV-J2ABFA
type: review-checklist
title: 'Review: Prose-check registry and engine with analyze prose'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`); sqlitestore passed on a rerun after a 10m timeout under host load; the rela-desktop chrome test needs the built SPA, which this checkout lacks
- [x] Lint clean (`just lint`); golangci-lint clean for the default and memorybackend tags (the sqlite and postgres passes timed out loading packages under host load; no changed file has a build tag); arch-lint clean
- [x] Comment lint gate clean (`just comment-lint`); fixed one doclink in internal/check/parse.go
- [x] Coverage maintained (`just coverage-check`); total 85.2%, internal/check 93.2%, internal/cmdexec 87.7%

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (server wiring moved out to PR 2)

**Review Responses:** RR-N9M1S4, RR-B1YIPG, RR-6M4FKP (significant, addressed);
RR-1TZAV3, RR-5BG69W, RR-GE0IWG, RR-QG0X7M, RR-MKBTML (minor, addressed);
RR-TYOC63, RR-68MJGU (minor, wont-fix); RR-6RNUFC (minor, deferred);
RR-YUTD9J (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `internal/metamodel/checkers_test.go`; manual `my task` load error.
2. PASS: `TestCheck_InputNameAndTypePlaceholder`, `TestCheck_UnsafeIDFallsBack`,
   `TestCheck_ValeTypeSections`; manual run: 115 procedure-only findings, none on tasks.
3. PASS: `parse_test.go` golden file; `TestResolve_Positions` incl. a span across a line break.
4. PASS: `TestRunWith_AcceptedExitCode`, `TestCheck_OneFailingCheckerDoesNotHideOthers`,
   `TestCheck_Timeout`; manual missing-config run.
5. PASS: `analyze_prose_test.go` (8 tests); manual run over 56 entities.
6. PASS: `TestAllowlist_ClassifiesEveryKey`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9LXX9Q

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: minimal commit message per the maintainer; the ticket holds the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)

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
