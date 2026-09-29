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

## Also in scope (reported by the BUG-1YN750 fix, #1708)

- `internal/entitymanager/cascadehost.go:182`: an automation-driven delete is face-blind; it writes one audit record and no per-face version capture.
- CLI `delete` and MCP `delete_entity` answer "entity not found" for a faced entity.

## Remaining zero-face reads in entitymanager (from BUG-58BL9I review)

- `findExistingRelationTarget` in `internal/entitymanager/core.go` and `internal/entitymanager/apply.go` read at the zero face and miss faced entities.
- `DeleteEntityFace` reads the face outside the transaction.
- `relation_grants` evaluation ignores faces.

## Stage 2 scope (moved from BUG-BZQQDP)

- Unlink over CLI, MCP and Lua, and a zero `RelationOptions{}`, call the
  default-tail `DeleteRelation`, so a content-scoped edge on a face cannot be
  removed through them. The data-entry relation routes already address the
  tail. Stage 2 makes the tail part of every relation call (`RelationKey` on
  `Get/Create/Update/DeleteRelation`, TKT-KQXVF7 PR 7), which fixes these
  callers.
