---
id: TKT-LJYWRA
type: ticket
title: Graph-wide as_of reads for views and exports
kind: enhancement
priority: low
status: backlog
---

## Description

A baseline (TKT-7LRUPB) can be listed and compared entity by entity from Lua,
but views, documents and exports always read the live graph. An auditor cannot
render "the SoA as it was at baseline/audit-2026".

Add graph-wide `as_of` reads (a version tag or a point in time) for views,
documents and exports, served from version history and gated like history reads
(deleted rows need `history:read`).

Out of scope of TKT-7LRUPB (brief G5). Related: reading a deleted entity's
tagged snapshot from Lua; `rela.version_by_tag` needs a live face today.
