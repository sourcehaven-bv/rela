---
id: IMPL-V83BHDY
type: implementation-checklist
title: 'Implementation: rela test v1 (2/4): injectable clock and determinism seams (mail transport, HTTP client, AI provider, output dir)'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (lua/seams_test.go, automation/clock_test.go, writerule/clock_test.go, memstore/clock_test.go, archguard/testclock_test.go)
- [x] Integration tests written (test full flow, not just units) (appbuild/seams_test.go drives real writes through NewInMemory; memrecipe tree-hash test now runs a write_file automation)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil and empty options keep defaults; temp dir inside root refused; blocked recipient stays blocked with an injected transport; AI seam only on writer runtimes)
- [x] Error handling in place (errors surfaced, not swallowed) (refused HTTP and AI reach the script as error tables; temp dir failures fail NewInMemory)

## Test Quality

- [x] Using fixture builders or factories for test data (writeSeamProject, probe, smtpTrap helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (through the appbuild integration tests; there is no CLI surface in this PR)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- TestWithClock_ReachesEveryWritePathClock: a pinned 2020-01-01 clock shows in {{today}}, rela.today in automation Lua, memstore UpdatedAt, a computed property, an enforce: write rule and a query scope.
- TestWithClock_ValidationUsesWorldClock: validation today() judges on the pinned day.
- TestNewInMemory_DefaultsKeepTheWorldOffTheNetwork: with SMTP mail.yaml and ai.yaml pointing at a counting listener, zero connections; HTTP and AI refused with ErrRealNetwork; recipients policy still blocks.
- TestNewInMemory_ZeroOptionsKeepTheDefaults, TestNewInMemory_InjectedSeamsWin, TestNewInMemory_DefaultOutputDirIsTemporary.
- TestNewInMemory_LeavesProjectTreeUntouched now includes a write_file automation and the tree hash stays identical.
- TestWallClockReads pins every remaining time.Now with a reason; its matcher self-test shows a new call is counted.
- just test, arch-lint, comment-lint, plimsoll pass.

## Quality

- [x] Code follows project patterns (check similar code) (options pattern, consumer-side seams on lua.ReadDeps, archguard allowlist shape)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (security review run; findings addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
