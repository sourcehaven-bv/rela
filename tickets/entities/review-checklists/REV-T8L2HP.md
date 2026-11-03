---
id: REV-T8L2HP
type: review-checklist
title: 'Review: Sessions 4/1: session config and message-driven transitions'
started: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race ./internal/...` default tag, sqlite-tag runs of the affected packages, and the full test run inside `just coverage-check`; postgres tests run in CI (no local DSN)
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): total 84.8%; internal/channels raised to 90.0% with direct session helper tests

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
- [x] All significant review-responses addressed: RR-CMHRTI
- [x] Self-reviewed the diff for unrelated changes: only the predicate ErrNilOperand change sits outside realtime, and it serves guard evaluation

**Review Responses:** design review 19 RRs (all addressed); code and security review RR-CMHRTI, RR-K4LKS8, RR-FTEJXW, RR-RUIBRQ, RR-K07ICO, RR-QU21P7, RR-N55621, RR-3BWQ1L, RR-0QQU0O, RR-SF0V9P (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-I4J0JC
- [x] Test evidence documented in implementation checklist: IMPL-ELF5HO

**Acceptance Status:**
All acceptance criteria in PLAN-I4J0JC PASS; each maps to a named test listed in IMPL-ELF5HO.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime.md, GUIDE-mcp-server.md
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-FMAX4H

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
