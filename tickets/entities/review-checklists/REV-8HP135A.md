---
id: REV-8HP135A
type: review-checklist
title: 'Review: Page the data-entry collection reads'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): touched packages only, per the brief: `go test -race` on dataentry, archguard, graphquerynaive, memstore, fsstore; sqlitestore with `-tags sqlite`; dataentry postgres tests on a scratch DB
- [x] Lint clean (`just lint`): golangci-lint on the touched packages, also with `--build-tags postgres`; `just arch-lint` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: floors are per package; no package lost tests and new code is covered by new tests)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent); rela-security-reviewer also ran (no findings above nit)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-6P0YNH4, RR-SDPT5H7, RR-R9XSG8X (significant,
addressed); RR-1CSABON, RR-O04RMYO, RR-8RICWBV, RR-JRDC1AM, RR-XARAI7Q (minor,
addressed); RR-5K6EUOT (minor, deferred to TKT-D3RAB0E); RR-W950H5A, RR-D2BEAK7,
RR-2UQD0G9, RR-00GK6IO, RR-XQ74LDB (nit, addressed); RR-130CE98, RR-D54MFV7
(nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 PASS: page-scoped (memstore, postgres), incoming-ordered and `?q=` list budget tests assert rows.
- AC2 PASS: `TestScopedHeaders_ByIDsIsTheFullReadRestricted`, `TestScopedHeaders_ByIDsKeepsTheWorldsFace`.
- AC3 PASS: `TestCommandExec_ListContextIsGatedAndRedacted`, BUG-2TPO2F goldens.
- AC4 PASS: `TestNoNewUnpagedReads`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: internal read path; `.claude/rules/collection-reads.md` updated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1UD42AL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (remaining `TODO(TKT-...)` notes point at the follow-up tickets)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after done, per the brief; CI is not watched here)

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
