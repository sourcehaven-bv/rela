---
id: REV-7PMQTEZ
type: review-checklist
title: 'Review: Remove unpaged list reads from request paths'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: whole-repo runs excluded at the user's request to limit machine load; every touched package passes with -race, and the postgres-tagged entitymanager, dataentry and pgstore suites pass on a local Postgres)
- [x] ~~Lint clean (`just lint`)~~ (N/A: whole-repo lint excluded at the user's request; golangci-lint is clean on each touched package, and `just arch-lint` is clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: whole-repo coverage run excluded at the user's request; new code has tests and no package lost tests beyond the removed EntityIDs path)

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

**Review Responses:** RR-NS5RNFK, RR-B1R06V8, RR-TGSKF7K (significant,
addressed); RR-4UCURBS, RR-QXXBQ85, RR-0PCXQ6H (addressed); RR-B3DJR5F,
RR-6JJ259C, RR-Z2WEN5L, RR-WRYHU2J (deferred); RR-TMELF0L, RR-7OW1F73
(wont-fix). The rela-security-reviewer reported no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestNoNewUnpagedReads` passes with the fixed entries removed.
2. PASS: the three `AssertScaleFree` budget tests pass.
3. PASS: entitymanager, appbuild and dataentry behavior tests pass; golden parity changed only in read-cost lines.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal change, no user-facing behavior)
- [x] ~~User-facing documentation updated~~ (N/A: internal change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the user asked not to push or open a PR)

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
