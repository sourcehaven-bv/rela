---
id: TKT-CBRLZWU
type: ticket
title: Role-relation roles count for menu and dashboard permission gates
kind: enhancement
priority: medium
effort: m
started: "2026-11-18"
completed: "2026-11-19"
status: done
---

## Description

A role conferred by a role relation (harbourmaster: `customer` held on an
account through `customer-of`) does not count in permission checks that have no
entity: navigation entries, page tabs, spaces and dashboard cards with a
`permission:`. Harbourmaster had to add a claim-mapped `portal` role to show the
customer menu.

These gates are UX filters, not access control ("the configuration is not a
secret"). For them, a permission should count when the principal holds a role
that grants it globally, or through a role relation edge on any entity.

## Acceptance criteria

- Nav entries, page tabs, spaces and dashboard cards (list and card data)
count a permission granted by a role the principal holds through any role
relation edge, honoring the edge's `when:` and the client ceiling.
- The answer is computed at most once per request with a bounded store read;
a read error counts no conferred role (fail closed).
- Every other entity-less permission check (commands, actions, documents,
history, config edit, comments, flow jobs, requires_permission) stays
global-only.
- Row and field gates are unchanged.
- Docs explain the rule.
