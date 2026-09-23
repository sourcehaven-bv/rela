---
id: FEAT-8LLX39
type: feature
title: Query-driven entity lists in the data-entry sidebar
description: Operators declare a query-backed navigation entry in data-entry.yaml; the sidebar lists the matching entities the principal may read as links.
status: proposed
---

## Summary

Operators can declare, in `data-entry.yaml`, a navigation group whose entries
are the result of a query over the graph (for example "Active projects": every
`project` with `status == active`). Each result renders as a sidebar link to
that entity. The list is per-principal and ACL-scoped.
