---
id: REV-8VA85T
type: review-checklist
title: 'Review: Realtime 3/3: channel service, HTTP API and live delivery'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` passes locally on the final branch; builds pass on the default, `postgres` and `sqlite` tags; the postgres DB tests run in CI only
- [x] Lint clean (`just lint`): golangci-lint, `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): coverage floors pass in CI on PR #1902; the stacked T3b PR runs the same gate in CI

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on the guard, the service and the HTTP/live part, each with a re-review
- [x] All critical review-responses addressed: RR-72EARV, RR-AOUEHB, RR-MZ18OI
- [x] All significant review-responses addressed: all significant findings are addressed, including RR-5TJ2OD (reload checks stored type records)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 101 review-response entities linked via
`has-review-response`. Open findings: none. Deferred: RR-WUNVER (minor: reload
still compares against the previous config for unrecorded types), RR-59QA3E
(nit: guard code lives in appbuild), RR-FDB8VE (nit). Won't fix: RR-6REA1Z,
RR-IAKGQ5, RR-KJ7FLN, RR-L5VSYZ, RR-N61P1I, RR-OVXRYQ (all nits). Each has a
recorded reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-VD856N
- [x] Test evidence documented in implementation checklist: IMPL-ODD5FN

**Acceptance Status:**
- Channel guard (block, detach, rename, type records at boot): PASS, `channelguard_internal_test.go`, `anchor_guard_test.go`, `anchor_guard_race_test.go`
- Channel service (post, read, message ops, signals, rate limits, role compile): PASS, `internal/realtime` tests and the `channelstest` harness on file, sqlite and postgres stores
- v1 HTTP routes under `_channels`: PASS, `channels_handler_test.go`, `channels_env_test.go`, `channels_budget_test.go`
- OpenAPI: PASS, `internal/openapi/channels_test.go` and the updated goldens
- Live delivery through topic rounds: PASS, `live_channels_test.go`

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime and GUIDE-acl-security, regenerated into `docs/realtime.md` and `docs/acl-security.md`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NAZORH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1899 merged; PR #1902 has green CI; the T3b PR stacked on #1902 is opened and monitored after the ticket reaches `done`; see TKT-UFV01M)

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
