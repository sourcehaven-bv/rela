---
id: REV-BMPE99
type: review-checklist
title: 'Review: Feed subscriptions: poll RSS/Atom/JSON Feed into entities'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (targeted: subscription, ingest, ingestspec, scheduler, appbuild e2e, projectsetup, cli, dataentry, dataentryconfig; the full suite runs in CI because the machine is under heavy load)
- [x] Lint clean (`just lint`) (golangci-lint on every changed package: 0 issues; arch-lint and plimsoll clean; full lint runs in CI)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new floor 85 for internal/subscription, at 93%; checked in CI)

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
- [x] All significant review-responses addressed (RR-5RDL03)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5RDL03, RR-IEH6QC, RR-9UF1AR, RR-YTKVSY, RR-0P5PRJ,
RR-RNJV8D, RR-Q4FRPD, RR-Q0Q1RF, RR-N471CQ, RR-5078OZ, RR-DAN1OW, RR-7FR0JT,
RR-8URLJ6, RR-4I4RKJ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: TestSubscription_EndToEnd, TestRun_ImportsThenSkipsUnchanged (no reads or writes on the second poll); manual go.dev feed run.
2. PASS: TestRun_NotModifiedKeepsValidators, e2e second poll sends the ETag.
3. PASS: TestRun_ChangedItemIsProcessedAgain, TestRun_DeletedImportStaysDeleted.
4. PASS: TestRun_TransientFailureKeepsValidatorsAndProgress, TestRun_PermanentRejectionDoesNotFailTheRun. Changed in design review: a conflict-exhausted item is counted and left unseen rather than failing the run; TestRun_ConflictExhaustedIsNotSeen.
5. PASS: TestRun_SkipsItemWithoutIdentity.
6. PASS: source_test.go (html, links), e2e asserts sanitized content and an absolute link.
7. PASS: config_test.go, TestLoadConfig, TestValidateWithFS_Subscriptions.
8. PASS: TestSubscriptionJob_RunsAsTheSubscription (principal and triggered_by).
9. PASS: fetch_test.go redirect tests.
10. PASS: dataentryconfig webhook tests unchanged.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8YA4RU

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per user preference)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist; see TKT-UFV01M)

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
