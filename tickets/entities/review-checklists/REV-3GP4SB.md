---
id: REV-3GP4SB
type: review-checklist
title: 'Review: Realtime 3/5: entity-change triggers and auto lifecycle'
started: "2026-10-19"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test passes locally on the touched packages of each slice; builds and vet pass on the default, `postgres` and `sqlite` tags; the postgres-tagged tests also ran against a local database, and the sqlite appbuild tests ran
- [x] Lint clean (`just lint`): golangci-lint on all four tag sets, `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): coverage floors pass in CI on PR #1933; the T5b PR runs the same gate in CI

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on each slice (T5a triggers, T5b auto.when, each with a re-review)
- [x] All critical review-responses addressed: none were raised
- [x] All significant review-responses addressed: all significant findings are addressed, including RR-37F02A (a lost postgres session no longer leaves two leaders) and RR-ISDJAJ (trigger posts land before an auto close)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 62 review-response entities linked via
`has-review-response`, from the design review and the reviews and re-reviews of
both slices. Open findings: none. All 62 are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-L2S743
- [x] Test evidence documented in implementation checklist: IMPL-JTFBUC

**Acceptance Status:**
- Triggers (enter, leave, change, when_closed, silent baseline, delete and restore): PASS, `internal/realtime/trigger_test.go`, `system_test.go`
- Delivery bounds (cap, cooldown, retries, recovery from a corrupt seen-set): PASS, `internal/realtime/trigger_test.go`, `seen_test.go`
- One leader across processes, takeover and lost session: PASS, `internal/appbuild/realtime_triggers_internal_test.go` and the postgres-tagged worker tests
- auto.when (once on change, reopen stands, from states, ordering after trigger posts): PASS, `internal/realtime/trigger_auto_test.go`
- Load checks (ephemeral post type, literals, hidden fields, faced types under face grants): PASS, `internal/realtimecfg` tests
- Wire fields author.system and triggered_by: PASS, `internal/openapi/channels_test.go` and the apiwire tests

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime, regenerated into `docs/realtime.md`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-IE76QM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1933 has green CI; the T5b PR carries the last slice and is monitored after the ticket reaches `done`; see TKT-UFV01M)

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
