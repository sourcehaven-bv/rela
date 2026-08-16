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

## Read gating (ACL)

MCP reads go through the same read-side ACL path as every other read shape.
The server does not hold a raw `store.Store`: it takes a narrow `GraphReader`
capability, and the wiring site supplies a visibility-wrapped reader that
resolves the principal from the call context. Row-level gating and field-level
`visible:` redaction therefore apply to MCP tools and resources exactly as they
do to the HTTP API — a hidden entity is absent, and a redacted property's value
is withheld.

The principal is stamped onto every tool-handler context by server middleware
and is required: `mcp.NewServer` returns an error rather than silently
degrading to an unauthenticated read. For `rela mcp` (stdio) the principal is
the OS user that launched the process, with `tool: "mcp"` — the same identity
recorded in the audit log below.

Because stdio MCP runs as a local user-launched process, this gating is
principally about consistency with the rest of the read paths rather than about
defending a network boundary. Serving MCP over HTTP is tracked separately as
Remote MCP part 2.

See [acl-security.md](acl-security.md) for the read-path rules these wrappers
enforce.

## Audit log

Every entity / relation write performed through MCP tools (including
`lua_eval` and `lua_run`) is recorded in `.rela/audit/YYYY-MM-DD.jsonl`
with `principal.tool: "mcp"`. The `principal.user` is the OS user that
launched `rela mcp` — *not* the LLM caller. MCP's wire protocol has no
notion of "user", so the host-process user is the right grain for
forensics: "alice ran an MCP-backed agent that did X".

Filter for MCP-driven changes:

```bash
cat .rela/audit/*.jsonl | jq 'select(.principal.tool == "mcp")'
```

See [audit-log.md](audit-log.md) for the full record schema.
