---
id: REV-I060VPR
type: review-checklist
title: 'Review: World switcher: labels and opt-in placement; hidden worlds; per-record toggle'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (Not run in full here: the machine is shared and the user asked to skip the pre-commit suite. Run: `go test` for metamodel, mcp, dataentry, dataentryconfig, configedit, apiwire; `npm run test:run` 396 files / 5989 tests; typecheck; e2e `faces-*` 44 passed, 2 skipped. CI runs the full suite.)
- [x] Lint clean: golangci-lint on the changed Go packages (0 issues), eslint on the changed frontend files (0 errors), `go-arch-lint check` OK
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (Not run here: it needs the full race suite. Every new branch has a test; CI checks thresholds.)

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

**Review Responses:** design review: 14 (all addressed). Code review: RR-EV4Q0ZQ,
RR-TCUJ9RH, RR-ALBBYB3 (significant, addressed); RR-5GKCJJD, RR-K8872GP,
RR-39PV7VI, RR-VN5OF02, RR-TZU1ZPL, RR-BFJT0R5 (addressed); RR-2K0EQEU,
RR-SN72M5D (wont-fix, with reasons).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1–AC7 PASS. Evidence in IMPL-O7OQTL0 (browser
check on a labelled copy of prototypes/worlds) and the named Go, vitest and
e2e tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GK1031B

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred to the user: pushing and opening a PR need their go-ahead.)

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
