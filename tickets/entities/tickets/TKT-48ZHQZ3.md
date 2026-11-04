---
id: TKT-48ZHQZ3
type: ticket
title: 'Entity-page row actions: delete owned rows with Undo and remove links'
kind: enhancement
priority: medium
status: backlog
description: Split from TKT-4Q2EIVZ; see body.
---

## Description

Add a per-row actions menu to entity-page sections: **Delete** for owned rows
(soft delete, toast with Undo through restore) and **Remove link** for linked
rows (confirm, no Undo). Split from TKT-4Q2EIVZ after its design review.

Design constraints from that review:

- Delete only when the section comes from one outgoing `owning` relation of
the entry (RR-LY9SC4O). Never on an incoming owning section.
- The server sends per-row `_actions.delete` and a section `unlink` verdict
through `translateVerb`, with contract and budget tests (RR-CQ6QJJX).
- The server sends a section descriptor `{relation, direction, anchor,
owning}` only when `EntryEdges` has one edge for the section (RR-6YIJRH1). No
actions without it.
- Remove link has no client-side relink Undo (RR-PABTYTY).
- Faced rows get no Undo (RR-3LDEAK1).
- Map `ErrOwningRule` on restore to 409 (RR-H7OWQ3H).
- Unlink addresses the entry with its face (RR-CJ7ODEZ).
- Negative ACs for every case above (RR-VIIGYMT).
