---
id: TKT-P9O4ME
type: ticket
title: Use the rela CLI instead of MCP for rela's own development
kind: chore
priority: medium
effort: s
status: done
---

## Description

Change rela to use the CLI for its own development instead of MCP. This reduces
context usage and helps develop the CLI further.

The `.mcp.json` server entries for the tickets and docs projects are removed and
`CLAUDE.md` now instructs agents to use `rela --project=tickets ...` and `rela
--project=docs-project ...` for all CRUD on tickets and docs.

## Rationale

- MCP tool definitions are loaded into every session regardless of whether that
session touches tickets. The CLI costs context only when it is called.
- Doing real work through the CLI surfaces its gaps. Defects an MCP-only
workflow would never reach show up during ordinary tasks.
