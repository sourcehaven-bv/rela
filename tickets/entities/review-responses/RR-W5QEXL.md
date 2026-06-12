---
id: RR-W5QEXL
type: review-response
title: 'IB-review PR939 #2: list/search/SSE/MCP enumeration not yet ACL-filtered'
finding: 'CISO IB-review on PR 939 (Laag, ter opvolging): list endpoints, sidebar counts, _position, /_search and SSE events are not yet ACL-filtered — any principal with API access can enumerate all entities of every type regardless of the per-entity read grants this PR enforces (CONTROL-5-15).'
severity: minor
reason: 'Acknowledged by the reviewer as explicitly documented and tracked: lists, sidebar counts, pagination headers and _position are closed by TKT-VMD8 / PR 949 (implemented, stacked on this PR, review-complete); /_search filtering, SSE per-subscriber visibility and MCP transport intersection (TKT-G3PPD) are the named follow-up tickets in TKT-VMD8''s follow-ups section. The ACL layer is not yet in production, so the residual window is the stacked-PR merge gap only.'
status: deferred
---
