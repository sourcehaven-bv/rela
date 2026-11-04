---
id: TKT-UXR6Y1B
type: ticket
title: Reject type-disallowed relation edges on every write surface
kind: enhancement
priority: high
effort: m
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

GitHub #1992. `PATCH /api/v1/{plural}/{id}` stores a relation edge whose
endpoint types the schema does not allow (for example `answers` declared `from:
[answer] to: [ask]`, linked answer→task) and answers 200 with a
`target_type_not_allowed` warning. `rela link` refuses the same edge.

The project owner decided that a relation's `from`/`to` types are a hard
contract: a type-disallowed edge is always rejected, on every write surface
(REST POST/PATCH and the per-edge relation endpoints, MCP, Lua, CLI). There is
no strict opt-in and no carve-out for existing edges. This supersedes the
type-mismatch part of DEC-HWZHA.

## Acceptance criteria

- REST POST and PATCH with a type-disallowed edge answer 422 with code
`source_type_not_allowed` or `target_type_not_allowed`, before any entity or
relation write, and record no write audit.
- The per-edge `POST /relations` endpoint answers the same codes.
- Canonical and inverse relation names, bounded and unbounded relations,
wrong source and wrong target are all covered by tests.
- `entity.RelationOptions.TolerateTypeMismatch` is removed, so MCP, Lua and
CLI share the manager's single check.
- Missing target, unknown meta and required meta stay warnings.
- A new decision supersedes DEC-HWZHA's type-mismatch classification; docs
and `internal/dataentry/CLAUDE.md` are updated.
- The repo's own projects contain no relation that violates its schema.
