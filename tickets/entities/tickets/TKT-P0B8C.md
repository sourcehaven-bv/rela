---
id: TKT-P0B8C
type: ticket
title: Show Lua error details for data-entry action failures
kind: enhancement
priority: medium
effort: xs
status: done
---

## Problem

When a data-entry action's Lua script fails, the user currently sees only a
generic message ("Action failed" or "3 failed, 2 succeeded") with a correlation
ID. The actual gopher-lua error — which already includes file:line and stack
traceback — is logged server-side but never reaches the UI.

## Goal

Surface the Lua error details to the user. Show a short error indicator
(preserving the existing UX), and on click open the existing Lua error modal
dialog with the full message, line number, and stack traceback for debugging.

## Notes

- The error is already produced by `RunActionString` in `internal/lua/runtime.go` and contains `file:line` + stack traceback.
- Currently dropped at `internal/dataentry/actions.go` `handleV1Action` — only `"action_failed"` + correlation ID are returned.
- Frontend display points: `frontend/src/composables/useListActions.ts`, `frontend/src/components/common/Sidebar.vue`.
- A modal dialog already exists for Lua errors elsewhere — reuse it.
