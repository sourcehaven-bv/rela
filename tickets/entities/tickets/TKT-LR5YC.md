---
id: TKT-LR5YC
type: ticket
title: Structured error reporting for Lua script failures
kind: enhancement
priority: medium
effort: m
status: done
---

When Lua scripts fail today (actions, documents, automations, lua_run,
lua_eval), the data-entry frontend shows generic toasts like "Action failed" or
"Failed to render document" while detailed error info (script path, Lua line,
stack, captured print() output, entity context) is logged server-side and
discarded. This makes scripts hard to debug for end users.

This ticket introduces a structured `ScriptError` envelope with inline source
context (±N lines around the failing line), captured print() output, Lua stack,
and correlation ID, returned by every Lua surface and rendered by a single
`<ScriptErrorPanel>` Vue component in data-entry.

## Acceptance criteria

- A Lua action/document/automation failure returns a structured envelope (script path, surface, entity id, sanitized args, Lua message, line, stack, captured print output, source slice, correlation id).
- Data-entry actions and documents render the envelope via a single shared `<ScriptErrorPanel>` (no more generic "Action failed" / "Failed to render document" toasts on script errors).
- MCP `lua_run`/`lua_eval` return the same envelope shape (JSON) so MCP clients see the same context.
- Captured `print()` output is preserved on failure (currently dropped in document.go).
- Automation script errors carry script path/identity (currently flattened to "script execution error: ...").
- Sensitive-looking arg keys are redacted before serialization.

## Out of scope

- "Last run" inspector / ring buffer of recent runs.
- Replay / "rerun with trace" mode.
- Lua debug-hook breakpoint UI.
- A new `/api/v1/_scripts/source` endpoint — source is inlined in the envelope.
