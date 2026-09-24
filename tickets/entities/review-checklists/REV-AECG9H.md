---
id: REV-AECG9H
type: review-checklist
title: 'Review: related() in views, next-action, CLI filter, validation, automation, state machine and ACL when:'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`, via `just ci`)
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

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-87VASW, RR-QZ6N76 (security, minor); RR-WHC2XQ,
RR-UFGKQS (significant); RR-QN8BOE, RR-93N356, RR-9NBFA4, RR-KJJ2CC, RR-WPG88P
(wont-fix), RR-CQIHCV (deferred); RR-PYZ63V, RR-9HZ570 (nits). Design review: 15
responses, all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: per-surface tests listed in PLAN-FHTWBQ.
2. PASS: budget tests at 10 and 50 rows (`TestViewCondition_RelatedIsSizeIndependent`, `TestQueryBudget_ListPageACLRelatedWhenIsSizeIndependent`, `TestViewCondition_RelatedWithTraversingGrantIsSizeIndependent`).
3. PASS: `TestQueryBudget_GetEntityACLRelatedWhenBindsOnce` baseline comparison.
4. PASS: `TestViewCondition_RelatedUsesReaderGate`, `TestGatedReads_ValidatorTraversalUsesCallerGate`, `TestGatedValidator_TraversalIgnoresHiddenEntity`.
5. PASS: `TestResolver_PrimedBindErrorDenies`, `TestViewCondition_RelatedUnsupportedFailsRequest`.
6. PASS: load-time validation tests per surface.
7. PASS: form condition refusal test.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HV27K1

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
