---
id: IMPL-H250G
type: implementation-checklist
title: 'Implementation: Enable contextcheck golangci-lint rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: pure ctx-threading refactor, no new logic; existing tests cover behavior)
- [x] ~~Integration tests written~~ (N/A: refactor; existing handler/integration tests exercise the threaded paths unchanged)
- [x] Happy path implemented — ctx threaded through all flagged chains
- [x] Edge cases from planning handled (helpers called from both ctx-aware and ctx-less sites threaded; iterator reads carry ctx; test entrypoints use Background)
- [x] Error handling in place — no error paths changed; ctx threading only

## Test Quality

- [x] ~~Using fixture builders~~ (N/A: no new test data introduced)
- [x] No hardcoded values added in assertions
- [x] Only ctx args added; no test values changed
- [x] ~~Interpolated values~~ (N/A)
- [x] ~~Property comparisons~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end — see evidence below
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1** (contextcheck enabled, comment removed): `.golangci.yml` now lists `- contextcheck` with the 4-line explanatory block removed. Verified via grep.
- **AC2** (`just lint` clean): `golangci-lint cache clean && just lint` → `0 issues.` No contextcheck violations, no new violations from other linters (gofmt/nolintlint/lll all clean).
- **AC3** (no behavioral regression): `just test` (race-enabled) → exit 0, all packages pass. The reverted-by-subagent incident was caught and fully reconstructed; affordances/dataentry/mcp/analysis/cli/lua/validation/scheduler tests all green.
- `just arch-lint` → `OK - No warnings found`.

Scope delivered: 101 violations resolved across internal/dataentry (58),
internal/mcp (22), internal/analysis (7), internal/affordances (5),
internal/scheduler (3), internal/cli (2), internal/validation/script/lua (1
each). Lua-runtime seams
(RunString/RunFileContent/RunActionString/RunValidationString) that bind ctx
once via `lua.WithContext` carry a targeted `//nolint:contextcheck` with
rationale — contextcheck can't follow ctx across the gopher-lua SetContext
boundary. ExecuteAction additionally gained a real `ctx` param (was previously
uncancellable).

## Quality

- [x] Code follows project patterns — ctx-first param convention; mirrors existing `outgoingRelationsCtx` precedent
- [x] Checked for DRY — no new duplication; lll-overflow lines hoisted rather than nolint'd
- [x] No security issues — auth-path predicate eval (affordances `passes`) now cancellable; verdict logic untouched
- [x] No silent failures introduced
- [x] No debug code left behind
