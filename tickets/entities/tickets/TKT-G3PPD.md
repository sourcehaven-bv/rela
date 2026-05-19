---
id: TKT-G3PPD
type: ticket
title: 'MCP transport intersection: filter tool list under ReadOnlyACL / Declarative'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Context

ACL v0 (TKT-GN5LN) gates writes at the EntityManager. That gate fires for MCP
too — `rela mcp` invokes `Manager.CreateEntity` and gets the same
`*acl.ForbiddenError`. But the MCP tool list itself is unfiltered: an LLM agent
sees `create_entity`, `update_entity`, `delete_entity`, etc., calls them, and
only learns they're denied at runtime.

The ACL design (`.ignored/acl-design.md`) calls for **transport-layer
intersection**:

> Agent permissions = intersect(user_capabilities, agent_scope), default-deny on writes.

The MCP transport should compute the intersection at registration time and only
expose tools the principal can actually invoke. The standard ACL check at
write-time remains as defense-in-depth.

## Scope

### v0-equivalent (this ticket)

When the server is constructed with `acl.ReadOnlyACL{}`:

- The MCP `list_tools` response omits every write tool (`create_entity`, `update_entity`, `delete_entity`, `create_relation`, `update_relation`, `delete_relation`, `rename_entity`).
- Read tools (`list_entities`, `show_entity`, `search_entities`, `trace_from`, `trace_to`, `analyze_*`) are unaffected.
- If an agent calls a write tool anyway (stale cache, manually crafted request), the MCP server returns a structured rejection rather than dispatching to EntityManager.

### v1-equivalent (deferred to v1 ticket)

When the server has a `Declarative` ACL:

- Intersect tool exposure with the principal's effective writes.
- Per-tool scope assignments via `acl.yaml: mcp_scopes` (sketch in design doc).
- Default agent scope = read-only even when the principal has writes.

## Acceptance criteria (v0 piece)

1. `rela mcp --read-only` (or equivalent flag wiring) returns a `list_tools` response missing all write tools.
2. An agent calling `create_entity` against a read-only MCP server gets a structured MCP error response, NOT a fall-through to the Manager.
3. Tests cover the registration-time filtering and the runtime defense-in-depth.
4. CLI tools (`rela create-entity`, etc.) are unaffected — they don't go through MCP.

## Open questions

- How does MCP learn the ACL mode? Same `appbuild.Option` plumbing as `rela-server`? Or a `--read-only` flag on `rela mcp`?
- Does MCP's `principal.Tool = "mcp"` justify a tool-keyed override even when the underlying ACL is `NopACL`? (Probably yes — agents are an attack surface; read-only-by-default is sensible.)
- Bigger v1 question: should `rela mcp` *always* default to read-only and require an explicit `--allow-writes`?

## References

- TKT-GN5LN (ACL v0 PR 1 — backend gate)
- DEC-RG878 (decision: MCP integration is transport-layer intersection)
- `.ignored/acl-design.md` §"MCP integration"
