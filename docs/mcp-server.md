# MCP Server

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
>   `metamodel.yaml` automatically — no cwd configuration is needed (or supported).
> - If both a local server and `.mcp.json` define `rela`, the local server takes priority.

The server communicates over stdio using JSON-RPC. It automatically discovers the project root
(by finding `metamodel.yaml`), loads the metamodel, and syncs the graph from markdown files.

## File Watching

The server watches `entities/`, `relations/`, and `metamodel.yaml` for changes. When files are
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

### View Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_views` | List available view definitions | (none) |
| `execute_view` | Execute a view for an entity | `name`, `id`, `format?` |

Views are declarative graph traversals defined in `views.yaml`. They efficiently gather all
related entities and relationships around a starting entity. Use `list_views` to discover
available views and `execute_view` to run one.

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
| `rela://view/{name}/{id}` | Execute a view for an entity (JSON) |

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
