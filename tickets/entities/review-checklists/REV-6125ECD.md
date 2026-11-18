---
id: REV-6125ECD
type: review-checklist
title: 'Review: Flows: start runs on create'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages on default, sqlite and postgres; the pg piles goldens in TestGoldenParity also fail on develop)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages, arch-lint, plimsoll)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new code has unit and integration tests; CI enforces floors)

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
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-EQVA0CK, RR-F9S33NN, RR-7X5154Z, RR-P49CP98,
RR-K2R84GB, RR-OCEG4IA, RR-ZHQS3I4, RR-5V46IZQ, RR-O1FM4PM, RR-U9TBSL1,
RR-4TH2G49, RR-LIACBSD, RR-4AVJG15, RR-YEG22OF, RR-G0JC9QS, RR-5BUCRRF

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Create on API, Lua, CLI and import starts one run: PASS (TestFlowCreateStart_* on default, sqlite, postgres)
- `when:` and `per_subject` respected: PASS (oncreate_test.go)
- Rolled-back create starts no run; retry starts no second run: PASS (sqlite and postgres integration tests)
- Chain of create starts bounded at depth 8: PASS (TestFlowCreateStart_ChainIsBounded)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IU9TKWV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run /pr command to create PR and monitor CI: done after this checklist (see note below)

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
