---
id: BUG-6XTX0G
type: bug
title: 'Remote MCP: faced entities are invisible to every read tool'
description: MCP and Lua reads run in the default world only so a faced entity with no default-face row is invisible; ID@face is not parsed.
priority: high
effort: m
why1: Every MCP tool and Lua binding reads with a zero store.WorldScope (the default world) and a bare-id GetEntity. A faced type has no default-face row, so its entities are absent. An ID@face address is looked up as a literal id.
why2: World selection (app.default_world, ?world=, ID@face parsing) exists only in internal/dataentry's HTTP middleware (attachWorld, resolveWorld, parseEntityRef) and is bound to ctx under an unexported key. /api/v1/_mcp is not world-capable, and the read handles from appbuild.Services.GatedReads take no world.
why3: When app.default_world moved server-side (QA finding 1) it was added as route middleware for the data-entry API rather than as an input to the gated read bundle. Remote MCP (TKT-UIR41P) then reused GatedReads, which has no world input.
why4: The zero WorldScope is defined as the pre-worlds default world, so a read path that forgets the world compiles and returns plausible results. Faceless types still work, and the MCP and Lua test fixtures declare no faces, so nothing failed.
why5: 'Systemic: world binding is opt-in per consumer with a silently compatible zero value, and no parity test asserts that web, MCP and Lua see the same entities for the same principal on a faced schema.'
prevention: Bind the world in the gated read bundle (appbuild.WorldBound) so every consumer of GatedReads inherits it instead of stamping it per call site. Add a faced-schema parity test over the MCP read tools and Lua reads.
status: done
---

## Report

Remote MCP (HTTP/OAuth) on a server with worlds/faces and `app.default_world`
set. Entity type `policy` declares faces `concept` and `adopted`; the default
world selects `[adopted, concept]`. `POL-001` has only an `adopted` face.

1. `list_entities type=policy` returns `[]`.
2. `show_entity POL-001` and `show_entity POL-001@adopted` return "entity not found".
3. `search_entities` on the title of POL-001 does not return it.
4. `lua_eval`: `rela.list_entities("policy")` is empty; `rela.get_entity("POL-001")` is nil.

The web UI shows the same entity to the same user. Faceless types work.
Reproduced on atlas (type `beleid`, `procedure`): list and type-scoped search
return nothing.

## Expected

A bare address resolves through the operator's default world, like the web read
API. An explicit `ID@face` address selects that face. Search results carry
`face` for faced entities (GUIDE-mcp-server).

## Impact

Agents cannot read faced content and may create a duplicate as a bare row.

## Ruled out

ACL: a bare `read: [policy]` grant covers every face (`acl/readquery.go`, "A
bare type grant reads EVERY face").

## Fix plan

1. `internal/worldreader`: a world-binding decorator over the gated entity
reader. It takes a per-call world source (scope, or "denied"). It stamps the
world onto `ListEntities` queries that name none. It resolves a bare `GetEntity`
through the world, typed so that the ACL face allowlist filters before the world
ranks. It serves an explicit `ID@face` literally.
2. A search decorator stamps the same world onto `search.Query`.
3. `appbuild.WorldBound(bundle, source)` wraps the reader, the searcher and the
Lua read handles of a `GatedReadBundle`. Tracer and validator are unchanged.
4. Remote MCP: `dataentry.MCPHost.ReadWorld` resolves `app.default_world` per
call (hot-reloaded config) and checks the world grant, as `resolveWorld` does
for the web API. `remoteMCPDeps` wraps the bundle with it.
5. MCP list and show output carry `face` for faced rows, as search already does.

## Added on review

An agent must be able to read content states other than the default world's
choice, such as a policy's concept face. The same PR adds:

- a `world` argument on `list_entities`, `search_entities` and
`show_entity`, checked against the caller's world grant like `?world=`; a denied
or unknown world is refused with an error;
- a `list_worlds` tool that names the worlds, their `select:` order, and
whether the caller may select each one;
- `other_faces` on `show_entity`: the entity's other faces the caller may
read, each with its `ID@face` ref.

The stdio server does not resolve worlds and accepts only `world: "default"`.

## Out of scope

- Web `/_search` ignores the world: separate bug and PR.
- A `world` argument on Lua reads, `list_relations` and the trace tools: TKT-PQMBL5.
- Tracer start-node lookup (`trace_from`/`trace_to`) on a faced id: follow-up.
- Create landing on the world's create face: FEAT-CRWFACE.
