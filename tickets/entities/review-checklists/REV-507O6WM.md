---
id: REV-507O6WM
type: review-checklist
title: 'Review: Reconcile endpoint POST /api/v1/_reconcile'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`, part of `just ci`)

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

**Review Responses:** design RR-VMZ3NM0, RR-IHCHB3J, RR-L0WVWGV, RR-P5HOOIS, RR-3FNL4AN, RR-BDBOB01, RR-YE7U6YW, RR-O8M3WR3, RR-T09PZLV, RR-06ZS7QG, RR-6QK6391, RR-KSR2XAK; code RR-JMX0MV2, RR-W3PJDEA, RR-JJ6NP8R (significant, addressed), RR-BJ19JE9, RR-OMM0JNN, RR-2WG27RY, RR-F12XM5E, RR-3DED5XQ, RR-0UPVRQR, RR-XGMMKR5, RR-2B3HGY8, RR-29V0J94 (addressed), RR-KT1PVVG (minor, deferred to step 7: shared gate key in header mode), RR-IE1H1W8 (minor, wont-fix: identity mismatch stays 500 by convention). Also resolved the deferred RR-MDMA4S5, RR-3RVT441, RR-ER381BQ. `just ci` passed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestReconcile_PrincipalsSeeTheirOwnRows (alice 5, bob 3, different fingerprints, no feature in bob's list); golden reconcile.* per role.
2. PASS: TestReconcile_HiddenFieldDoesNotReachTheHash.
3. PASS: fold check in TestReconcile_PrincipalsSeeTheirOwnRows.
4. PASS: TestReconcile_Paging, TestReconcile_PagingPastAPrefix.
5. PASS: TestReconcile_RefusesBadRequests.
6. PASS: TestReconcileGate; manual 503/429 with Retry-After against the tickets project.
7. PASS: TestReconcile_NoIdentityFailsClosed (401).
8. PASS: router walk probe, TestReconcile_CrossOriginRefused, TestReconcilePath, golden coverage.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-LGUAD6G

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step after done; CI status is tracked on GitHub)

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
