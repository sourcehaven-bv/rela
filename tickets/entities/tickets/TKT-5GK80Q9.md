---
id: TKT-5GK80Q9
type: ticket
title: 'Scoped relations: within and on_change'
kind: enhancement
priority: medium
effort: l
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Add scoped relations to the schema. A relation can declare `within: <relation>`
and `on_change: clear|reject`. The target of a scoped edge must be owned
(through an `owning:` relation, one hop) by the entity that the source's
`within` relation points to. Example: a todo's `under_heading` must point at a
heading owned by the project that the todo's `in_project` points to.

## Scope

- Load-time checks in internal/metamodel.
- Enforcement on the entitymanager write path, in the write's transaction.
- Data-entry form: picker filtered by the unsaved scope value; dependent field
cleared or flagged on change; disabled while scope is empty.
- Docs in docs-project.
