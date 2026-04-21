---
id: REV-I4AEM
type: review-checklist
title: 'Review: Introduce RootedFS type and pilot on state.FSKV'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — all packages green, state=100%, storage=86.3%
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — 72.6% total, floor thresholds satisfied

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed — 6 significant findings, all status=addressed
- [x] Self-reviewed the diff for unrelated changes — diff is tight, only `rooted.go` (new), `rooted_test.go` (new), `state.go/_test.go` (rewrite), and three call-site migrations (workspace.go, dataentry/app.go, two test helpers)

**Review Responses:** 15 review-responses created from cranky-code-reviewer
findings.

- **Significant (6, all addressed)**: RR-MGASV (doc overstates invariant), RR-WD0NA (relativize swallowed errors), RR-HQDEL (silent nopState on error), RR-AF1CU (Windows backslash bug), RR-0GM44 (lazy root creation), RR-E80D4 (nil fs accepted).
- **Minor (7)**: RR-LF2JF (Windows reserved names, addressed), RR-16FM9 (Root() escape hatch removed), RR-1GJVI (fuzz test added), RR-3ON0U (fallback removed, resolves it), RR-HMU95 (config.validateName drift, deferred to TKT-K3YYE), RR-OOX3Y (Walk order, wont-fix — not part of contract).
- **Nits (3)**: RR-1VEUZ (WalkAll '.' doc, addressed), RR-E3IJ3 (error style, wont-fix), RR-KXIBR (4-pass resolve, wont-fix — premature optimization).

All critical + significant resolved. Remaining 3 wont-fix/deferred are justified
with reasons.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1** (RootedFS exists, cleans/absolutizes root, rejects empty): PASS — `TestNewRootedFS_CleansRoot`, `TestNewRootedFS_RejectsEmptyRoot`, `TestNewRootedFS_RejectsNilFS` (added during review), `TestNewRootedFS_ResolvesRelativeRoot`.
- **AC2** (resolve accepts/rejects): PASS — 8 accept cases + 15 reject cases (up from 11 pre-review, now includes colon-anywhere and 4 Windows reserved names).
- **AC3** (method shapes delegate): PASS — tests for WriteFile/ReadFile/Remove/Rename/MkdirAll/ReadDir/Stat/Open/Walk/WalkAll. `TestRootedFS_WriteFile_CreatesParentDirs` (added) verifies the auto-mkdir behavior.
- **AC4** (Walk/WalkAll callback receives keys): PASS — `TestRootedFS_Walk_ReturnsKeys`, `TestRootedFS_WalkAll_FromRoot`, plus `filepath.IsAbs` assertion on every seen path.
- **AC5** (state.FSKV uses RootedFS, validateKey gone): PASS — diff shows validateKey removed; FSKV.Put now a one-liner; 3 production call sites + 2 test call sites migrated.
- **AC6** (just ci green): PASS — lint 0 issues, tests pass, coverage 72.6%, build clean. Arch-lint local run: 2 pre-existing `.ignored/` notices unchanged from develop (not a regression). Fuzz: 2.9M executions in 10s, no escapes.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via has-docs~~ (N/A: internal refactor, no user-facing docs)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

Package-level doc comment in `internal/storage/rooted.go` documents the pattern
for future internal contributors.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — grep `rooted.go` confirms zero
- [x] Ready for another developer to use — public API is `NewRootedFS(fs, root)` + the keyed methods; package doc explains the invariants

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — all code checks green (Test/E2E/Frontend/Fuzz/Lint/Build/Architecture/Docs/Demos/Vulnerability), ticket-validation check will go green on transition-to-done
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/552
