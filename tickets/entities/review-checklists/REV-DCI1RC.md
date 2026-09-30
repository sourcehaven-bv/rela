---
id: REV-DCI1RC
type: review-checklist
title: 'Review: History restore of a deleted entity fails when its status is past the entry state'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test -race ./..., plus -tags sqlite, -tags memorybackend and just test-postgres)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; arch-lint and plimsoll clean)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (two rounds, cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TPOL99, RR-HIX5DL, RR-3FMCW6, RR-0RU1H1, RR-KSGK52,
RR-SQ6Z9P, RR-L1OCE1, RR-C47ZPT, RR-5PY4AB, RR-CK851X, RR-ZTD5AZ, RR-BY78QI,
RR-605O1R (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS restore past the entry state, faceless: TestTransition_RecreateEntity_RestoresPastEntry
- PASS restore past the entry state, faced: TestTransition_FacedRestorePastEntry, TestHistoryRestore_DeletedFacePastEntryState
- PASS ordinary create in a non-entry state refused: TestTransition_FacedRestorePastEntry/create_is_refused, TestTransition_IllegalEntryOnCreateIs422
- PASS guards bind: TestEnforceRestore, TestTransition_RecreateEntity_GuardedStateIs403, TestHistoryRestore_PastEntryStateNeedsGuard
- PASS exemption unreachable from ordinary create: TestRecreateOnlyFromRestore

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug, not an enhancement)
- [x] User-facing documentation updated (GUIDE-cli-reference, GUIDE-metamodel, GUIDE-postgres-backend)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug, not an enhancement)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create against faces-intrinsic after done)

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
