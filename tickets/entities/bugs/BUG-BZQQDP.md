---
id: BUG-BZQQDP
type: bug
title: Relation reads and writes mishandle faced endpoints
description: FilterRelations hides faced edges under ACL, relation create/GET to a faced target 404, and relation writes lack the face gate.
priority: high
effort: m
status: backlog
---

## Problem

Reported by the face-awareness inventory, not yet verified:

- `visibility.PolicyReader.FilterRelations` hides every edge touching a faced entity under an ACL (`policyreader.go:161`). This affects MCP relations and show, and Lua `get_relations`.
- Relation create to a faced target and the relation-target GET return 404 (`write_handler.go:1028`, `relation_read_handler.go:111-116`).
- Clone and relation writes lack the face gate, which gives a 403-versus-404 existence oracle.
- `DeleteRelation`, unlink and `RelationOptions{}` act on the default tail only.

## Expected

Relation reads and writes resolve faced endpoints by address and apply the face
gate with the uniform 404.
