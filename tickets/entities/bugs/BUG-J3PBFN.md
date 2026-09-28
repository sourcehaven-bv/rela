---
id: BUG-J3PBFN
type: bug
title: CLI, MCP, Lua, automation and importer writes read the zero-face row
description: Delete, create_relation and import outside the data-entry app look up or create zero-face rows on faced types.
priority: medium
effort: m
status: backlog
---

## Problem

Reported by the face-awareness inventory, not yet verified. Write paths outside
the data-entry app read the zero-face row:

- Entity and relation delete over CLI, MCP and Lua (`cli/delete.go:26`, `mcp/tools_entity.go:421`, `lua/runtime.go:1976,2042`).
- Automation `create_relation` loses faced targets and the tail (`autocascade/runner.go:249-263`, `cascadehost.go:115`).
- The importer writes bare rows on faced types (`importer.go:80,450-458`).

## Expected

These paths accept `ID@face` and never create or look up a zero-face row for a
type that declares faces.
