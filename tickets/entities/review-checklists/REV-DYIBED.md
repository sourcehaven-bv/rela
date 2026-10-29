---
id: REV-DYIBED
type: review-checklist
title: 'Review: Guarded branching transitions per message type'
started: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on the changed packages, sqlite-tag runs, and the full run inside `just coverage-check`; postgres tests run in CI
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 85.4% total, all package floors pass

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: 2 significant fixed: shared-target trigger close mark (resolveMark matches the job flag, with a test) and composer type switch now drops the form decision as documented
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-R422O1 RR-3246CN RR-OPSAK9 RR-59H14G RR-FCUUFV
RR-2Z06QB RR-DZR09L RR-9UYQXQ RR-EI4YU6 RR-50NBNG RR-X2NVXK RR-23RYXL RR-GTZRNM
RR-L8ZP3E RR-L7480G RR-G5TJKC RR-YYTC1K RR-EDUX0N RR-C4RJ7E RR-U0WQ31 RR-3760CL
RR-Z5JCAI RR-3S4MLB RR-KJSCP4 RR-BI84Z0 RR-UMCUN7 RR-TESKRA RR-824OK1

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-N0AM2O
- [x] Test evidence documented in implementation checklist: IMPL-6F2LR0

**Acceptance Status:** All acceptance criteria in PLAN-N0AM2O PASS; each maps to
a named test listed in IMPL-6F2LR0.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-XE5WEA
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-XE5WEA

## Final Checks

- [x] Commit message explains the why, not just what: minimal message per user preference; the ticket carries the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

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
