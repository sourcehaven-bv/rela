---
id: RR-QOIXK
type: review-response
title: RelationOptions.Content *string change ripples through 9 call sites including MCP/Lua public APIs
finding: |-
    Plan says 'audit during implementation; mechanical updates'. Reviewer enumerated 9 sites: dataentry handlers, workspace manager, mcp/tools_relation.go (MCP tool API), cli/link.go, lua/runtime.go (Lua public API). MCP/Lua sites change semantics for AI clients and scripts: what does `content: ""` mean — clear or leave alone? Plan doesn't specify. Recommendation: enumerate now, decide MCP/Lua semantic, or use `ClearContent bool` to avoid the cascade.

    From design-review: F4.
severity: significant
resolution: 'Plan Layer 0 enumerates 9 call sites + 1 stub in a table. MCP tool: helper `nilIfEmpty` preserves today''s leave-alone behavior on `content: ""`; tool docs note to pass `null` or omit the field. Lua: deliberate semantic change — `update_relation({content=""})` now clears (matches user intuition); documented in lua API reference.'
status: addressed
---
