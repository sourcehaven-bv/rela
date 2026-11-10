---
id: REV-26Y86ZT
type: review-checklist
title: 'Review: Flows M2: step forms and run UI'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

Evidence: `just coverage-check` (race, whole module) passed; golangci-lint on
the changed packages reports 0 issues; frontend typecheck and eslint clean;
`just comment-lint`, `just arch-lint` and `just plimsoll` clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer ran. No
critical findings. Significant, all addressed: RR-WJAS92W, RR-LTS1N1M,
RR-5GR71V3. Minor and nit addressed: RR-KAB5HUB, RR-TYONDR4, RR-F118SGT,
RR-VWJACBI, RR-5WJH5ZH, RR-P0U4VQP. Deferred to TKT-FWQZGAM with reasons:
RR-6KWVWY8, RR-WKJIKAW, RR-QI6OYY9, RR-ZD27306, RR-X6G1SF5, RR-WO1JSPZ.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Run list and inbox row-gated, batched per page, pinned by a budget test:
PASS. `TestFlowControl_Tasks` and `TestFlowControl_RunList` check the gate per
principal; `TestFlowControl_TasksBudget` pins the store reads flat between 10
and 50 runs for a global and a local role holder.
- E2E test completes a sample flow through the SPA: PASS.
`e2e/tests/flows.spec.ts` starts a run, completes the step through the form and
sees the run finish.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-1O28ER0

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
