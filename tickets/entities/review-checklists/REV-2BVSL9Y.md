---
id: REV-2BVSL9Y
type: review-checklist
title: 'Review: Store query contract: or, not and ordered comparisons'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (changed packages on memstore, fsstore, sqlite and postgres; the full suite runs in CI)
- [x] Lint clean (golangci-lint on the changed packages with default, postgres and sqlite tags; arch-lint; plimsoll)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (new code is covered by new tests; the floor check runs in CI)

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

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-K8RT5G1, RR-FV5CTS8)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-K8RT5G1, RR-FV5CTS8, RR-EMBHCS9, RR-B7GBBJP,
RR-LJR5MZC, RR-CA8JVO7, RR-NUUX203, RR-2L4VE4L, RR-RSQJGIU (wont-fix: the field
does not exist)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: or, not and ordered comparisons answered by the store on every backend (storetest GraphCond suites; RunGraphDifferential on sqlite and postgres).
- PASS: number and string comparison is JSON-safe (OrderString, OrderInteger; TestIntegerExprAgreesWithGo).
- PASS: hidden fields keep the Go path; LowerScope unchanged (dataentry and mcp pushdown tests).
- PASS: LowerCondition builds Where (TestLowerCondition_AgreesWithEvaluator, 300 random conditions).
- PASS: follow-ups filed (TKT-W0DYCEC, TKT-FKE3VP3, TKT-4I0XFKO); fallbacks left to TKT-9BKVQ9I and TKT-I3MGCRT.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-A8JR2UF

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after done, as the note below explains)

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
