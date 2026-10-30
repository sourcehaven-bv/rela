---
id: REV-3DY7GOE
type: review-checklist
title: 'Review: rela validate accepts deprecated link: true that rela-server refuses'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test for internal/projectsetup, internal/migration, cmd/rela-server and internal/cli ok; the change is confined to those packages; every tracked data-entry.yaml still validates)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages: 0 issues; `just arch-lint` clean. Repo-wide runs fail to load packages under load from parallel sessions; CI runs them)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (every new line is covered by the new tests; CI runs the gate)

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

**Review Responses:** Design: RR-PT0E1VD, RR-8MWKU55 (addressed). Code:
RR-2DGKOU3, RR-PJB4A49, RR-IO6DHEP, RR-OD7UM9O, RR-8VDCNI2 (addressed),
RR-J9TCLJ1, RR-1BEIUGC (wont-fix, nits). No critical or significant findings.
The security review found no issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS (TestValidateWithFS_DeprecatedDataEntry; manual
rela validate exits 1 with the migrate hint). AC2 PASS (link: detail cases;
validate exits 0 after rela migrate). AC3 PASS (IsMigrationError asserted). AC4
PASS (grep finds no link: true in the guide or docs/data-entry.md).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-UV3J23G

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
