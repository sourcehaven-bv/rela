---
id: TKT-9KZGJO
type: ticket
title: World-aware analysis and cross-world search grouping (Step 5)
kind: enhancement
priority: medium
effort: l
status: backlog
---

Design doc §7, §10.

- Cardinality rules declare `must_hold_in: [worlds]`; absent → default world only, so existing metamodels behave identically. Subjects and edges come from the same resolved graph; violations labeled by world (one rule can fail in one world and pass in another — both reported).
- Orphans run in the default world; states never appear in orphan output.
- Search: query scope is a world (one hit per entity within it); editor surfaces query several worlds, grouped by entity in the search API (SPA/MCP/CLI share it; `limit` counts entities). Grouping runs AFTER the per-world read gate — annotating "also matched in editorial" for someone who may not read that world is an existence-and-content oracle.

Gated on the cardinality consolidation and the Step 2 world scope.
