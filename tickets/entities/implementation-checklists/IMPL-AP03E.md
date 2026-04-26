---
id: IMPL-AP03E
type: implementation-checklist
title: 'Implementation: Structured error reporting for Lua script failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented end-to-end
- [x] All edge cases from planning handled
- [x] No silent failures (errors surfaced, not just logged)

**Implementation summary:**

Phase 1: `internal/lua/scripterror.go` + tests — `ScriptError` type,
`BuildScriptError` (typed `*glua.ApiError` extraction, source slice with
traversal-resistant FS access, recursive arg redaction, captured-output
redaction & cap).

Refactor mid-phase: replaced regex-on-error-string with PCall message-handler
approach (gopher-lua issue #46). Runtime captures typed `lua.Debug` frames via
`GetStack`/`GetInfo`; `BuildScriptError` consumes them directly. ~70 lines of
regex parsing deleted.

Phase 2: actions surface end-to-end. `script.Engine.ExecuteAction` wraps Lua
errors as `*lua.ScriptError`. `internal/dataentry/script_errors.go` (new) holds
the envelope + per-request loopback gating via `r.RemoteAddr`. Handler does
`errors.As` to branch — Lua errors → 422 envelope, contract errors → existing
500.

Frontend: TS type, Pinia store with replace-latest + focus-restore,
`<ScriptErrorPanel>` (presentational), `<ScriptErrorDialog>` (role=alertdialog,
Esc, click-outside, focus management) mounted in `App.vue`. `Sidebar.vue` action
handler routes via `isScriptError(err)`. Axios interceptor passes the envelope
through to catch handlers.

Phase 3: documents, automations, MCP rolled in. Document renderer attaches
captured `print()` via new `(*ScriptError).WithCapturedOutput()`. Automation
engine plumbs automation name into `LuaToExecute`; workspace tags inline blocks
as `automation:<name>`. MCP `lua_run`/`lua_eval` return JSON envelopes via
`NewToolResultError(jsonString)` to preserve `IsError`. `DocumentsPanel.vue` and
`DocumentView.vue` dispatch via the same store as actions.

## Manual Verification

- [x] Feature tested end-to-end manually
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

**Verification evidence:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 — actions return envelope | PASS | `TestHandleV1Action_ScriptError_DegradedForNonLoopback` and `_FullForLoopback` assert 422 + envelope shape; existing `TestHandleV1Action_OpenRedirectRejected` (contract failure) continues to assert 500 |
| 2 — documents return envelope | PASS | renderScript wraps via `WithCapturedOutput`; api_v1 handler routes via `errors.As` |
| 3 — MCP envelope same shape | PASS | `TestHandleLuaEval_ReturnsScriptErrorEnvelope` + `_PreservesIsErrorFlag` |
| 4 — automation script identity | PASS | `TestLuaAutomation_LuaExecutionError` updated to assert `automation:<name>` shape |
| 5 — captured print preserved | PASS | `TestExecuteAction_ScriptError` asserts `before` is in CapturedOutput |
| 6 — frontend panel renders | PASS | `scriptError.test.ts` (5 cases); manual: dialog opens, Esc/click-outside close, focus restored |
| 7 — non-script errors unchanged | PASS | Sidebar / DocumentsPanel / DocumentView fall through to existing toast when `!isScriptError(err)` |
| 8 — args + output redacted | PASS | `scripterror_test.go` covers key denylist (password, api_key, authorization, etc.), value-shape (JWT, long hex, long b64), nested redaction, length truncation |
| 9 — loopback gating | PASS | `TestHandleV1Action_ScriptError_DegradedForNonLoopback` asserts non-loopback gets degraded shape (no source/captured/stack); `_FullForLoopback` asserts full shape |

## Quality

- [x] Code follows project patterns
- [x] Errors surfaced, not just logged
- [x] Lint passes
- [x] Tests pass
- [x] Coverage check passes

**Quality evidence:**

- `just test` — all packages green
- `just lint` — 0 issues
- `just coverage-check` — PASS at 74.0%
- Frontend: `npm run typecheck`, `npm run test:run` (513 tests), `npm run build` all clean
- All checks via the pre-existing patterns: capability bundles (`ReadDeps`/`WriteDeps`), no service locator, no repository abstractions

## Deviations from plan

Two intentional simplifications during implementation:

1. **No `RunFileWithCapture` helper.** The plan called for new helpers in `internal/lua/runtime.go` to pair captured output with the run. On reading the call sites it turned out the buffer is already in the caller's scope at every relevant point — the engine just stops dropping it on err. Added `(*ScriptError).WithCapturedOutput()` instead so callers (notably the document renderer) can attach the captured bytes after the fact. Net deletion vs. the plan.

2. **`effects.Errors` stayed `[]string`.** The plan called for `[]error` to expose typed `*lua.ScriptError` to consumers. On audit, the public API field `UpdateResult.AutomationErrors` is asserted by ~13 test sites and not currently read for typing — it's just rendered as text. The stringified envelope (`automation:<name>:2: <message>`) carries the script identity that was missing before. Worth revisiting if anyone needs typed access downstream; flagged in the Phase 3 summary.

Both deviations are documented in the phase summaries; they reduce surface area
without losing functionality.
