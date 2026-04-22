---
id: TKT-CGPYW
type: ticket
title: Generalize rela.mode to all Lua execution contexts
kind: enhancement
priority: low
status: backlog
---

## Problem

TKT-CGBVW introduces `rela.mode = "document"` for data-entry document scripts,
but leaves `rela.mode` unset (`nil`) in all other contexts (CLI `rela script`,
`rela flow`, data-entry actions, scheduled tasks, validation). This keeps the
blast radius small for the initial rollout.

Once there's a concrete need (e.g., a script that wants to behave differently
when run from the scheduler vs. the CLI), we should set `rela.mode` in every
context with a stable string:

| Context                | Proposed `rela.mode` |
|------------------------|----------------------|
| `rela script <file>`   | `"script"`           |
| `rela flow <file>`     | `"flow"`             |
| data-entry action      | `"action"`           |
| data-entry document    | `"document"` (already shipped in TKT-CGBVW) |
| scheduled task         | `"scheduled"`        |
| validation rule        | `"validation"`       |
| `lua_eval` / `lua_run` | `"mcp"` or similar   |

## Open questions

- Should validation rules get `rela.mode` at all? Validation runs with a reader runtime and `io.Discard` stdout — a script that branches on mode inside a validation rule is almost certainly a design smell.
- What string for MCP `lua_eval` vs. `lua_run`? Keep them distinct or unified?

## Scope

- Set `rela.mode` in each entry point that builds a Lua runtime.
- Document the full set of values in GUIDE-lua-scripting.
- Add a table-driven test covering every context.
