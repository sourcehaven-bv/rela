---
id: BUG-RIJR6R
type: bug
title: 'Remote MCP: lua_eval/lua_run bypass read ACL; search_entities returns unreadable hits'
description: Remote MCP offered lua_eval/lua_run/lua_list with unrestricted reads, and search_entities returned id/type/title of hits the caller cannot read. Both bypass the read-side ACL for remote principals.
priority: high
why1: The remote wiring passed svc.LuaWriteDeps() (unrestricted reader) and svc.Searcher() (raw index) into the MCP Deps, and registerTools registered every tool on every transport.
why2: TKT-BDG8U9 reused the stdio Deps shape and swapped only Store/Tracer/Validator for GatedReads(); GatedReads had no searcher, and nothing marked the Lua and search deps as transport-sensitive.
why3: The TKT-UIR41P decision (AC 7, TestRemoteMCP_NoLuaTools) lived only in ticket prose; the planned test was never written, so nothing failed when the exclusion was missed.
why4: 'Tool registration was opt-out by default: a tool is exposed remotely unless someone removes it, so omissions fail open.'
why5: Security acceptance criteria are tracked as ticket text rather than as named tests that must exist before a ticket can close.
prevention: Lua tools are now opt-in per server (WithLuaTools), so a new networked wiring fails closed. GatedReads carries the searcher so the gated bundle is complete. Remote tool-set and search tests pin both in cmd/rela-server.
status: done
---

## Problem

Found in code review of develop fa09ecf96.

1. Remote MCP (`/api/v1/_mcp`) registered `lua_eval`, `lua_run` and `lua_list`. The wiring passed `svc.LuaWriteDeps()`, whose reader is `visibility.Unrestricted(store)`. A remote script could read rows and `visible:`-hidden fields its caller cannot. TKT-UIR41P decided (AC 7, PLAN-O8KMBQ `TestRemoteMCP_NoLuaTools`) that Lua tools are not offered remotely, but that was never built. TKT-BDG8U9 wrongly described them as ACL-gated.
2. `search_entities` used the raw `svc.Searcher()` and returned id, type and index title of hits the caller cannot read. It also returned rows that matched only on a hidden property (the TKT-GGQ0JT oracle).

## Fix

- `mcp.WithLuaTools()` makes the Lua tools opt-in. Only the stdio wiring (`rela mcp`) passes it. The remote server gets no Lua deps and does not register the tools; a call fails as an unknown tool.
- `appbuild.GatedReadBundle.Searcher`: row scope from the ACL read query, the hidden-field match filter, and the face filter, resolved per call from the ctx principal. Raw searcher under NopACL; refuses when the gate cannot be built.
- `search_entities` hydrates every hit through `Deps.Store` and builds the summary (title, status) from that entity, so a hidden row or hidden title cannot reach the result even with an ungated searcher.
- Tests: `TestRemoteMCP_NoLuaTools`, `TestRemoteMCP_SearchExcludesUnreadable`, `TestRemoteMCP_SearchRedactsHiddenTitle`, `TestRemoteMCP_SearchDropsHiddenFieldMatch` (cmd/rela-server), `TestNewServer_LuaToolsAreOptIn`, `TestACL_SearchEntities_OmitsHidden` (internal/mcp), `TestGatedReads_Searcher` (internal/appbuild).
