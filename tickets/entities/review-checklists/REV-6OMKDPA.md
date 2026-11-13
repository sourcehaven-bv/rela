---
id: REV-6OMKDPA
type: review-checklist
title: 'Review: Two-way sync: latest/local/remote rules and relation push (9b)'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): race runs of remotesync, affordances, visibility, scheduler, cli, openapi, entitymanager; full dataentry run; golden corpus regenerated
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, plimsoll and arch-lint clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): remotesync 80%, visibility 89%, scheduler 84%, all above their floors

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky and rela-security-reviewer
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed: RR-OUNS3IW
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OUNS3IW (significant, addressed); RR-XPBYH92 [security], RR-ESA4SS4, RR-2P9AYKM, RR-YN0ZLZC, RR-1WOLSGF (addressed); RR-ENVA2RY, RR-BGUZIUA, RR-0VBWE8N, RR-UKLW3VN, RR-KUA77YM (deferred); RR-YA1NHI9, RR-0LS7IBA (wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-72JZL32)

**Acceptance Status:**
1. PASS: rule tables (`TestLatest`, `TestDecide`, `TestRules_*`, `TestParse_RulePrecedence`) and the two-instance `TestRemotePush_RelationsAndLatest`.
2. PASS: `TestUpdated_EntityOnlyWhenNothingHidden`, `TestUpdated_RelationOnlyWhenNothingHidden`, `TestUpdated_RelationWithheldAfterHiddenKeyCleared`, `TestRules_LatestFallsBackToHold` (time_withheld).
3. PASS: `TestLatest` and `TestRules_LatestFallsBackToHold` (future, before sync, same time).
4. PASS: `TestRules_LatestRemoteWinsAndKeepsTheLocalValue`, `TestDismiss_RefusesAHeldEntry`, `TestDismissRemote`, the CLI `--dismiss` tests and the two-instance test.
5. PASS: `TestRelationPatch_IfMatch`, `TestRelationPatch_IfMatchIgnoresHiddenMeta`, entitymanager `relation_cas_test.go`.
6. PASS: `TestRelationPush_*` (create, meta update, conflict, latest, refusal, gated type, unreadable endpoint, failed merge).
7. PASS: godoc in `source.go` and `push.go`; PLAN-4E6K1U8 text fixed.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: docs/remote-sync.md, ACL guide, API reference, OpenAPI
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QLUERI6

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR follows done, see TKT-UFV01M)

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
