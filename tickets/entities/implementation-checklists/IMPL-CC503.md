---
id: IMPL-CC503
type: implementation-checklist
title: 'Implementation: Introduce RootedFS type and pilot on state.FSKV'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/storage/rooted_test.go` (18 tests covering constructor, resolve, all method shapes, Walk/WalkAll, invalid-key short-circuit via spy)
- [x] Integration tests written (test full flow, not just units) — `internal/state/state_test.go` exercises FSKV through MemFS end-to-end
- [x] Happy path implemented — `internal/storage/rooted.go`; `state/state.go` migrated
- [x] Edge cases from planning handled — all 12 rejection cases from validateKey copied to resolve; WalkAll separate from Walk (no magic-string asymmetry)
- [x] Error handling in place (errors surfaced, not swallowed) — every resolve error is returned to caller; underlying FS errors pass through unchanged

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests exercise filesystem primitives — MemFS construction is the fixture. `newTestRooted(t)` is the local helper.)
- [x] No hardcoded values in assertions when object is in scope — `TestRootedFS_Resolve_Accepts` uses `rfs.Root()` instead of hardcoded `/root`
- [x] Only specifying values that matter for the test — key strings are the subject of the tests; other state (perms, content) uses simple placeholders
- [x] Interpolated values constructed from objects, not hardcoded — see `filepath.Join(rfs.Root(), k)` pattern
- [x] Property comparisons use original object, not hardcoded strings — rename round-trip reads back the written value, not a hardcoded literal

## Manual Verification

- [x] Feature manually tested end-to-end — unit + integration tests cover the full stack (RootedFS → MemFS; FSKV → RootedFS → MemFS)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — all validation rejection cases have explicit tests

**Verification Evidence:**

All ACs verified by passing tests:

- **AC1** (`RootedFS` exists, constructor cleans/absolutizes root, rejects empty): `TestNewRootedFS_RejectsEmptyRoot`, `TestNewRootedFS_CleansRoot`, `TestNewRootedFS_ResolvesRelativeRoot` — all PASS.
- **AC2** (`resolve` accepts/rejects correct keys): `TestRootedFS_Resolve_Accepts` (8 cases) + `TestRootedFS_Resolve_Rejects` (12 cases) — all PASS.
- **AC3** (all method shapes delegate correctly): `TestRootedFS_WriteFile_ReadFile`, `TestRootedFS_Remove`, `TestRootedFS_Rename_BothArgsValidated`, `TestRootedFS_MkdirAll_ReadDir`, `TestRootedFS_Stat`, `TestRootedFS_Open`, `TestRootedFS_WriteFile_InvalidKey_DoesNotTouchFS` (spy confirms short-circuit) — all PASS.
- **AC4** (Walk/WalkAll callback receives keys, not abs paths): `TestRootedFS_Walk_ReturnsKeys`, `TestRootedFS_WalkAll_FromRoot` — all PASS. Callback-path assertion checks `filepath.IsAbs` is false for every seen path.
- **AC5** (state.FSKV uses RootedFS; validateKey removed): diff of `internal/state/state.go` shows validateKey gone, replaced with single `*RootedFS` field; all 5 state tests PASS. Call sites in `workspace.go`, `dataentry/app.go`, 2 test files migrated.
- **AC6** (`just ci` green): `just lint` → 0 issues; `just test` → all packages PASS; `just coverage-check` → PASS (72.5% total, state=100%, storage=86.3%); `just build` → PASS. Arch-lint local run unchanged from develop baseline (2 pre-existing `.ignored/` notices, not a regression).

## Quality

- [x] Code follows project patterns (check similar code) — matches `state.validateKey`'s rule set verbatim; constructor pattern mirrors `NewMemFS`/`NewSafeFS`; `Root()` accessor follows the `FS.Getwd` precedent
- [x] No security issues introduced — `resolve` is the single path-validation barrier. Rules match the existing `state.validateKey`. Walk callback remapping prevents leaking backing FS paths
- [x] No silent failures (errors logged AND returned) — every error path returns to caller
- [x] No debug code left behind — reviewed diff; no print/log statements added
