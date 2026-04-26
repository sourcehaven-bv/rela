---
id: DOCS-KQLKQ
type: docs-checklist
title: 'Documentation: Structured error reporting for Lua script failures'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Public Go types have doc comments (`ScriptError`, `BuildScriptError`, `BuildInput`, `StackFrame`, `SourceLine`, surface constants, `WithCapturedOutput`, `Runtime.ErrorFrames`, `ScriptErrorEnvelope`, `ScriptErrorPolicy`, `writeV1ScriptError`)
- [x] Frontend types documented in `frontend/src/types/scriptError.ts`
- [x] Non-obvious WHY comments added (path-cleaning belt-and-braces, X-Forwarded-For omission, captured-output-bypasses-MCP)

## Project documentation

- [x] ~~CLAUDE.md "Don't do this" entry~~ (N/A: the existing list covers package boundaries; adding "don't return raw Lua errors from a new surface" would be aspirational guidance with no companion rule. Skipping per CLAUDE.md "no premature abstractions".)
- [x] No README.md changes — N/A: rela's README is a brief project overview; envelope shape is an internal detail.

## External documentation

- [x] ~~User-facing docs (Lua scripting reference)~~ (N/A: no user-facing docs site exists yet for the Lua scripting surface; the rela-docs MCP graph has a `lua-scripting` concept but no rendered guide. When that guide lands, it should mention the structured error panel.)
- [x] ~~OpenAPI / API docs~~ (N/A: data-entry has no published OpenAPI spec; the existing `writeV1Error` shape isn't documented either, so adding only the new envelope would be inconsistent.)

## Notes

- The new HTTP envelope shape (`{"error":"script_error", ...}`) is an additive change — clients that don't recognise it fall through to existing error-handling paths.
- Loopback gating ensures non-loopback callers receive a degraded envelope (path + message + line + correlation_id only) by default. Operators who explicitly need full detail in production-ish bind modes can set `ScriptErrorPolicy.AlwaysFullDetail = true`; the wiring is via `App.scriptErrorPolicy` and is intentionally left at the zero-value for now.
