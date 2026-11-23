---
id: TKT-2MDJRGW
type: ticket
title: 'Sankey view for data-entry (sankeys: config, layout, view)'
kind: enhancement
priority: medium
effort: l
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Summary
Add a `sankeys:` view to `data-entry.yaml`: columns of entity types, relations
as ribbons.

- Config, validation, nav entry (`sankey:`), `_sankeys/{id}` endpoint gated like the gantt (row-gate, redact, where, cap).
- Own layout library (crossing minimiser, ribbons), no d3.
- Vue view with hover highlight.
- Detail panel: a click selects the node (`?selected=`) and opens the shell overlay panel.
- Pips: `pips:` per type draws related entities as dots. One batched read of the pip type and one relation query per type; ACL-gated, with a post-gate `+n` count.
- Docs: GUIDE-data-entry "Sankeys".

Deferred: sankey page tabs, node weight from a property, live refresh.
