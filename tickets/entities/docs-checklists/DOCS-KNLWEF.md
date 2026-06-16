---
id: DOCS-KNLWEF
type: docs-checklist
title: 'Documentation: Expose request principal to Lua runtime (TKT-5U6NRR)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Public API documented — the new `rela.principal` binding has a contract comment at its registration in `internal/lua/runtime.go` (read-only, populated from `callerCtx()`, unstamped fallback, why it's not a spoofing vector). The `freezeTable` helper documents the proxy/__index/__newindex/__metatable mechanism.
- [x] Non-obvious WHY captured — the comment cites PLAN-XKMJ AC13 and explains that attribution derives from `callerCtx()` in the write bindings, never from this table, so reading identity cannot forge a write.

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-lua-scripting.md` — added a `rela.principal` row to the Context table (read-only `{user, tool}`, request principal, frozen, not a spoofing vector). Regenerated `docs/lua-scripting.md` via `just docs`.

## External Documentation

- [x] ~~User guide / tutorial~~ (N/A: a Lua API field for script authors; the Context-table reference is the right surface).
- [x] ~~Changelog~~ (N/A: project has no separate changelog; PR description carries the summary).
- [x] CLAUDE.md — no new project-rule entry warranted; the read-only-principal contract lives with the binding and the spoofing test.

## Verification

- [x] Docs accurate against current code — the documented shape matches `TestPrincipalReflectsContext` / `TestPrincipalIsReadOnly`.
- [x] `just docs-check` passes (generated docs in sync with source).
