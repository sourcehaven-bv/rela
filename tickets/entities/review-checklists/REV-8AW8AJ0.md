---
id: REV-8AW8AJ0
type: review-checklist
title: 'Review: Two-way sync: send deletes with guards'
started: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (touched packages: dataentry, remotesync, cli, reconclient, dataentrywire; frontend typecheck + vitest; full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages: 0 issues; arch-lint OK; plimsoll clean on touched packages)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the disk is full, so coverage runs in CI; new code is covered by deletes_test.go and the e2e test)

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

**Review Responses:** cranky-code-reviewer and rela-security-reviewer.
Significant, all addressed: RR-2432MQR, RR-4ELY7CO, RR-SZTE2ZT, RR-VE812QV,
RR-W3RM3MU, RR-AS8DJEM. Minor addressed: RR-J6H75DJ, RR-R35YGOZ,
RR-RODNPI9, RR-TK6RR7S. Minor deferred with reasons: RR-32SR5E5,
RR-8HKXKSG, RR-D10MUE3, RR-DN1C61K, RR-FA92E95, RR-WMW35OX, RR-6IV4WJO,
RR-TGQDV1J. Minor wont-fix: RR-K299282.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestRemoteDelete_EndToEnd/sends` deletes the relation and the
   entity on a real data-entry server; `TestDeletes_SendsLocalDelete`.
2. PASS: on_gone delete is covered by the remotesync unit tests
   (`run_test.go`, `deletes_test.go` remote-delete rules).
3. PASS: `TestDeletes_RemoteEditHolds`, `TestDeletes_RemoteEditRules`;
   e2e `sends` holds the delete as `local_deleted`.
4. PASS: `TestDeletes_RemoteDeleteOfEditedRow` and `...Rules`.
5. PASS: `TestDeletes_GuardHoldsOnlyDeletes`, `TestDeletes_Window`,
   `TestDeletes_EmptiedHereIsRefused`; e2e guard then `--allow-gone 2`.
6. PASS: `delete_ifmatch_test.go` for entity and relation.
7. PASS: `TestDeletes_RemoteEditHolds` (TakeLocal, TakeRemote),
   `TestDeletes_ResolveRemoteSurvivesGuard`, `TestDeletes_RemoteDeletedRelation`.
8. PASS: `TestDeletes_NeedsPush`, `TestDeletes_OffByDefault`, `TestDeletes_TurnedOff`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NMJ76NU

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened right after this checklist; see TKT-UFV01M)

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
