---
id: GUIDE-mcp-server
type: guide
title: "MCP Server"
status: published
order: 9
audience: advanced
summary: "AI assistant integration via MCP"
---

Rela includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server
that exposes its full capabilities to AI assistants. This allows tools like Claude Code, Cursor,
and other MCP-compatible clients to query, create, and analyze entities and relations directly.

## Quick Start

Start the server manually (for testing):

```bash
rela mcp
```

### Claude Code Setup

**Option 1: `claude mcp add` (recommended)**

```bash
claude mcp add rela -s local -- /path/to/rela mcp
```

This stores the server configuration privately per-user per-project in `~/.claude.json`.

**Option 2: `.mcp.json` (for sharing via git)**

```json
{
  "mcpServers": {
    "rela": {
      "command": "rela",
      "args": ["mcp"]
    }
  }
}
```

Project-scoped servers defined in `.mcp.json` require interactive approval on first use.

> **Notes:**
>
> - Claude Code launches MCP servers with the project directory as cwd, so `rela mcp` finds
>   `schema.yaml` automatically — no cwd configuration is needed (or supported).
> - If both a local server and `.mcp.json` define `rela`, the local server takes priority.

The server communicates over stdio using JSON-RPC. It automatically discovers the project root
(by finding `schema.yaml`), loads the metamodel, and syncs the graph from markdown files.

## File Watching

The server watches `entities/`, `relations/`, and `schema.yaml` for changes. When files are
created, modified, or deleted, the graph is re-synced automatically and connected clients are
notified via `notifications/resources/list_changed`. Changes are debounced with a 200ms window.

## Tools

### Entity Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_entities` | List entities with optional filtering | `type?`, `where?`, `limit?`, `offset?` |
| `show_entity` | Get full entity details with relations | `id` |
| `search_entities` | Full-text search across entities | `query`, `type?`, `limit?` |
| `create_entity` | Create a new entity | `type`, `properties`, `content?`, `id?` |
| `update_entity` | Update entity properties or content | `id`, `properties?`, `content?` |
| `delete_entity` | Delete an entity and its relations | `id`, `cascade?` |

**Filtering with `where`:**

The `list_entities` tool supports property filter expressions:

```text
status=accepted
priority!=low
status=draft,proposed
```

### Relation Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_relations` | List relations with optional filtering | `type?`, `from?`, `to?` |
| `create_relation` | Create a relation between entities | `from`, `type`, `to`, `content?` |
| `delete_relation` | Delete a relation | `from`, `type`, `to` |

### Graph Tracing Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `trace_from` | Trace all dependencies from an entity | `id`, `max_depth?` |
| `trace_to` | Trace upstream dependencies to an entity | `id`, `max_depth?` |
| `find_path` | Find shortest path between two entities | `from`, `to` |

### Analysis Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `analyze_orphans` | Find entities with no connections | `type?` |
| `analyze_cardinality` | Check relation cardinality constraints | (none) |
| `analyze_properties` | Validate entity properties against schema | (none) |
| `analyze_validations` | Run custom validation rules | (none) |

### Schema Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_metamodel` | Get the full metamodel definition | (none) |
| `list_entity_types` | List entity types with property schemas | (none) |
| `list_relation_types` | List relation types with constraints | (none) |

### Utility Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `refresh` | Force re-sync the graph from disk | (none) |
| `export` | Export entities/relations | `format` (json/yaml/csv), `type?` |

## Resources

Resources expose rela data as readable URIs.

| URI | Description |
|-----|-------------|
| `rela://metamodel` | Full metamodel schema (JSON) |
| `rela://entity/{type}/{id}` | Single entity with properties and relations |
| `rela://relation/{from}/{type}/{to}` | Single relation |

## Prompts

Prompts provide pre-built workflows that combine data retrieval with LLM instructions.

### analyze-traceability

Analyze traceability coverage for an entity. Returns the entity details, full trace tree
(upstream and downstream), and asks the LLM to evaluate completeness.

**Arguments:** `id` (required)

### review-orphans

Review orphan entities and suggest connections. Returns the list of orphans and available
relation types, then asks the LLM to suggest which relations should be created.

**Arguments:** `type` (optional, filter by entity type)

### summarize-project

Generate a project overview. Returns entity/relation counts by type, metamodel overview,
and analysis summary.

**Arguments:** none

### review-entity

Review an entity for completeness and quality. Returns the full entity, its property schema,
and validation results.

**Arguments:** `id` (required)

## Remote MCP (over HTTP)

Everything above describes `rela mcp`, the **local stdio** transport. A
deployed `rela-server` can serve the same tools over HTTP so a hosted
assistant reaches your project without a local checkout.

It is **off by default**. Enable it with `-mcp` (or `RELA_MCP=1`):

```bash
rela-server -mcp \
  -jwt-issuer https://idp.example.com \
  -jwt-audience rela-prod \
  -jwt-jwks-url https://idp.example.com/.well-known/jwks.json
```

The endpoint is `POST /api/v1/_mcp`.

### The JWT flags are mandatory

`-mcp` **refuses to start** without `-jwt-issuer` / `-jwt-audience` /
`-jwt-jwks-url`. This is not a style preference:

An MCP client is not a browser and sends no `Origin`, so the endpoint has to
be exempt from the same-origin (CSRF) check the rest of `/api/` gets. That
exemption is only sound while rela verifies a bearer token *itself*. In
header-identity mode (`-principal-header`, or nothing at all) an
unauthenticated request resolves to the user `unknown` — combined with the
CSRF exemption that would be an unauthenticated, internet-reachable write
surface. Refusing at startup is the only place to catch it, because the
downgrade would otherwise show up per request, long after anyone reads the
startup log.

### What a remote caller can do

Exactly what their ACL grants — no more, and no less than the same person
gets through the web UI:

- Every read goes through the same ACL gate as `/api/v1/...`. Two callers
  hitting the same endpoint see different rows.
- Every write is authorized and audited as the **requesting** principal, with
  `principal.tool: "mcp"`.
- A denied entity is indistinguishable from a nonexistent one.

Note the current scope: **every tool is exposed remotely**, including
`lua_eval` and `lua_run`. Those run in a sandboxed interpreter with no OS
libraries and are ACL-gated like everything else, so they are not an escape
hatch — but if you would rather a new tool were opt-in per transport, that
allowlist is not built yet.

### Differences from stdio

- **Stateless.** Protocol revision `2026-07-28` removes sessions, and the
  Go SDK only reaches it in stateless mode. `GET` and `DELETE` return 405;
  only `POST` carries messages.
- **No change notifications.** `resources/list_changed` has no stateless
  equivalent, so there is no file watcher. Clients re-read on demand and
  always see current data, because every read hits the store.
- **No IdP auto-discovery yet.** RFC 9728 Protected Resource Metadata is not
  served, and the 401 carries a `WWW-Authenticate` challenge only when your
  assertion header is literally `Authorization`. Point your client at the IdP
  by configuration.

## Audit log

Every entity / relation write performed through MCP tools (including
`lua_eval` and `lua_run`) is recorded in `.rela/audit/YYYY-MM-DD.jsonl`
with `principal.tool: "mcp"`.

**Over stdio**, `principal.user` is the OS user that launched `rela mcp` —
*not* the LLM caller. The stdio protocol has no notion of "user", so the
host-process user is the right grain for forensics: "alice ran an
MCP-backed agent that did X".

**Over HTTP**, `principal.user` is the JWT-verified subject of the request,
so the record names the actual caller. Asserted roles and org are carried
too, exactly as for a web-UI write.

Filter for MCP-driven changes:

```bash
cat .rela/audit/*.jsonl | jq 'select(.principal.tool == "mcp")'
```

See [audit-log.md](audit-log.md) for the full record schema.
