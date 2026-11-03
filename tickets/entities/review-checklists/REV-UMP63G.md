---
id: REV-UMP63G
type: review-checklist
title: 'Review: Realtime 3/4: channel lifecycle, actions and promote-to-graph'
started: "2026-10-19"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test passes locally on the touched packages of each slice; builds and vet pass on the default, `postgres` and `sqlite` tags; the channel store and promote tests also ran against a local postgres database and sqlite
- [x] Lint clean (`just lint`): golangci-lint on all four tag sets, `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): coverage floors pass in CI on PRs #1926 and #1927; PR #1932 runs the same gate in CI

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on each slice (T4a retention, T4b lifecycle, T4c actions and promote)
- [x] All critical review-responses addressed: the critical findings (for example auto-close re-closing a reopened channel) are addressed
- [x] All significant review-responses addressed: all significant findings are addressed, including RR-MYS3LA (promote refuses refs the actor cannot read)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 80 review-response entities linked via
`has-review-response`, from the design review and the three slice reviews. Open
findings: none. All 80 are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-UKLXH9
- [x] Test evidence documented in implementation checklist: IMPL-TZSS6L

**Acceptance Status:**
- Retention sweep, entity-delete policy and purge: PASS, `internal/realtime/sweep_test.go`, `purge_test.go`, `internal/appbuild/channelguard*_test.go` and the `channelstest` harness on file, sqlite and postgres stores
- Lifecycle operations (close, reopen, archive, delete with retention): PASS, `internal/realtime/lifecycle_test.go` and the dataentry handler tests
- Actions, promote and release: PASS, `internal/realtime/actions_test.go`, `internal/appbuild/realtime_promote_test.go`
- OpenAPI paths for lifecycle and actions: PASS, `internal/openapi/channels_test.go`

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime, regenerated into `docs/realtime.md`
- [x] Docs-checklist marked as done

**Docs Checklist:**

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PRs #1926 and #1927 merged; PR #1932 carries the last slice and is monitored after the ticket reaches `done`; see TKT-UFV01M)

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
