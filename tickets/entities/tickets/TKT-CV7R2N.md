---
id: TKT-CV7R2N
type: ticket
title: Add operator-defined entity type conversions
kind: enhancement
priority: high
effort: xl
completed: "2026-10-15"
status: done
---

## Description

Allow a user with the conversion ACL grant to change an existing entity to an
operator-approved target type while retaining its history, comments, links,
and mapped relations. Include metamodel declarations, API preview and write
operations, required-field input, and the frontend conversion flow.

## Acceptance

- Operators configure source-to-target conversions and explicit property,
  relation, and face mappings.
- The API checks conversion ACLs and previews required fields and relation
  effects before the write.
- Conversion rekeys the entity and keeps old links, comments, and history
  available under the new identity.
