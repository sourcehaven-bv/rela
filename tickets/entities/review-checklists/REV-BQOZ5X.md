---
id: REV-BQOZ5X
type: review-checklist
title: 'Review: Session composer: offer only message types that apply in the current state'
started: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on the changed packages, sqlite-tag runs, and the full run inside `just coverage-check`; postgres tests run in CI
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 85.3% total, all package floors pass

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
- [x] All significant review-responses addressed: none raised in code review; the 3 design-review significant findings were folded into the plan
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-24ESXU RR-DDE518 RR-3YSOUQ RR-PU9JUP RR-XYK9S2
RR-CQ6LFI RR-4G9DL8 RR-OVZABK RR-JL8QGD RR-X02I61 RR-TG8KVC RR-NBM6CM RR-3EXS9B
RR-QJABHE RR-OK2KRM RR-MG8GD1 RR-3WJRLV RR-Z3YUEH RR-4VMWSL RR-YGEDLA RR-Z5L4WY
RR-OSV1AH RR-QQXTTI RR-ZCCVFY RR-MP7MZT RR-BQG3SO RR-B8E1RJ RR-VYAJYD

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-FUP8IS
- [x] Test evidence documented in implementation checklist: IMPL-4VD83F

**Acceptance Status:** All acceptance criteria in PLAN-FUP8IS PASS; each maps to
a named test listed in IMPL-4VD83F.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-8I87ZH
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8I87ZH

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
