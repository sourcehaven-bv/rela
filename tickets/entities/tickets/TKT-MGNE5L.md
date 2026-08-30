---
id: TKT-MGNE5L
type: ticket
title: 'mcp.Server round 2: lua/schema/resources/prompts handlers (38 → ~25)'
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of [[TKT-N0IKN9]], second step of the `mcp.Server` arc after
[[TKT-YUETL7]] (49 → 38). Stacked on TKT-YUETL7's branch.

## What

**Pure structural extraction, no behavior change, no wire-visible change.**

- **`luaHandler`** (−3): `handleLuaEval`, `handleLuaRun`, `handleLuaList`
(tools_lua.go) over `{lua.WriteDeps, lua cache, projectRoot}` — deps used by
NOTHING else in the package, so this also stops other handlers from seeing the
Lua capabilities. capabilities_posture_test.go + tools_lua_test.go pin behavior.
- **`schemaResourceHandler`** (−6): the schema tools (`handleGetSchema`,
`handleListEntityTypes`, `handleListRelationTypes`, tools_schema.go) and the
resources (`handleReadMetamodel`, `handleReadEntity`, `handleReadRelation`,
resources.go) share the same deps `{store GraphReader, meta}` — one merged type.
PRESERVE the aliasing: `toolGetSchema()` and `toolGetMetamodel()` both register
the same handleGetSchema.
- **`promptHandler`** (−4): the four prompt handlers (prompts.go) over
`{store GraphReader, meta, tracer}`. golden_test.go pins prompt output.

`registerTools`/`registerResources`/`registerPrompts` stay on Server; affected
registration lines re-point. principalMiddleware untouched — no principal
threaded into any handler struct. Ratchet directive 38 → ~25.

## Done when

plimsoll with lowered directive; full suite + `-race ./internal/mcp/` green;
dispatch/golden/capabilities-posture tests unchanged;
arch-lint/comment-lint/lint clean.
