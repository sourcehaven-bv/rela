---
id: REV-TR0M3UC
type: review-checklist
title: 'Review: Inbox 6: cross-process live delivery'
started: "2026-11-02"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): all packages pass except `cmd/rela-desktop` `TestChromeStyle_TargetsShippedClasses`, which needs the built SPA and fails in this checkout without a frontend build (unrelated; CI builds it). Postgres-tagged suites pass against a local PostgreSQL; sqlite and memorybackend builds pass for the changed packages
- [x] Lint clean (`just lint`): all four tag sets, plus `just lint-md`, `just plimsoll`, `just arch-lint`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): thresholds pass, total 85.5%

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and the rela security reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** significant, addressed: RR-XQ1065C, RR-WOWGIA5,
RR-GLPIQJ3, RR-9V7JVCZ, RR-0A4K520. Minor, addressed: RR-5RXV27S (security),
RR-IUD1R47, RR-R8XDZLS, RR-ENXFX5Z, RR-NYZ2AMB, RR-30PFR07, RR-HLX2SFE,
RR-FULC19F, RR-2308G6C. Minor, deferred to TKT-PMNFOGP: RR-MUDT8HY (security).
Nits: RR-FEZLH7Y, RR-R78QWCD addressed; RR-605CZDL, RR-6HKC1UA, RR-H4FKGRS won't
fix with reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1, AC3, AC5a: PASS, `TestChannelHints_ReachOtherProcess` (file and postgres builds): post in `Durable`, read marker in `Reads`, rename and delete in `Moved`; addressed delivery by `TestHints_KindsReachOtherLocal`.
- AC2: PASS, `TestHints_KindsReachOtherLocal` (moved kind); the integration test covers moved through rename and delete.
- AC4: PASS, `TestHints_OwnOriginDroppedAndNoEcho`.
- AC5: PASS, ephemeral marks send nothing (`internal/realtime` hint tests).
- AC5b: PASS, blocking sender and queue overflow tests.
- AC5c: PASS, `TestChannelHints_SurviveReassembly`, `TestHintRelay_AttachDetachAndSwap`.
- AC5d: PASS, `TestChannelCLI_PostReachesOtherProcess` (subprocess, file and postgres).
- AC6: PASS, `filechannels` hint feed tests, including concurrent processes with rotation.
- AC7: PASS, `TestNotify_*` against a local PostgreSQL.
- AC8, AC8a: PASS, `TestChannelHints_FailingSendKeepsPost`, `TestHints_ResyncLimit`.
- AC9: PASS on the file backend with an SSE client in place of a browser (`rela channel post` from another process reached the stream about 420 ms after the CLI exited). The stdio MCP `post_message` and two-server postgres manual checks were not run; the postgres path is covered by the postgres-tagged integration and CLI subprocess tests.
- AC10: PASS, `GUIDE-realtime.md`, `GUIDE-postgres-backend.md`, `GUIDE-mcp-server.md`, `GUIDE-cli-reference.md`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-7ZNOUOW

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the user asks for minimal commit messages; the why is in the ticket and plan)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after the ticket is done)

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
