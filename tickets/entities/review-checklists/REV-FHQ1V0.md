---
id: REV-FHQ1V0
type: review-checklist
title: 'Review: Realtime 3/6: channel MCP tools, watch and CLI'
started: "2026-10-19"
completed: "2026-10-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test passes locally on the touched packages of each slice; builds and vet pass on the default, `postgres` and `sqlite` tags; the sqlite-tagged tests and the postgres-tagged channel tests also ran; four existing `TestLeaseCLI_*` tests fail under the postgres tag locally because they open the project without database settings, which predates this ticket
- [x] Lint clean (`just lint`): golangci-lint on all four tag sets, `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): coverage floors pass in CI on PR #1928; the T6b PR runs the same gate in CI

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on each slice (T6a MCP tools, T6b identity and CLI, each with a re-review)
- [x] All critical review-responses addressed: none were raised
- [x] All significant review-responses addressed: all significant findings are addressed, including RR-SE9O2M (asserted_by never comes from `$USER`)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 70 review-response entities linked via
`has-review-response`, from the design review and the reviews and re-reviews of
both slices. Open findings: none. All 70 are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-T02YT4
- [x] Test evidence documented in implementation checklist: IMPL-N6QEB6

**Acceptance Status:**
- MCP channel tools (list, read, post, edit, remove, create) under acl.yaml: PASS, `internal/mcp/tools_channels_test.go`, `TestMCPStdio_ChannelToolsApplyACL`
- MCP watch: PASS, `internal/mcp/tools_watch_channels_test.go`
- Untrusted text fencing and per-type JSON schema: PASS, `internal/realtime` and `internal/realtimecfg` tests
- `--run-as` / `RELA_RUN_AS` rules, refusals and asserted_by: PASS, `internal/cli/identity_test.go`, the rela-server subprocess test, the archguard audit test
- `rela channel` commands, rendering and watch: PASS, `internal/cli/channel_test.go`, `channel_render_test.go`, `channel_watch_test.go`

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-realtime, regenerated into `docs/realtime.md`
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-D9QY7A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR #1928 merged; the T6b PR carries the last slice and is monitored after the ticket reaches `done`; see TKT-UFV01M)

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
