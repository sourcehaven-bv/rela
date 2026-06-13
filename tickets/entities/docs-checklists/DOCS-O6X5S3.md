---
id: DOCS-O6X5S3
type: docs-checklist
title: 'Docs: ACL read-side: SSE /api/v1/_events per-type gating'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc: `sseEvent` (the three variants + RelationChange marker), `broadcastEntityChange` / `broadcastRelationChange`, `runSSELoop` (gating + coalescing + fresh-gate re-derive), `entityTypeVisible` (fail-closed), `freshReadGate`, `handleSSE` (rewritten — entity:changed {type}-only)
- [x] Frontend: `EntityEventData` ({type} only), `useEvents` event list, dirtyFormRegistry comment

## Project Documentation

- [x] GUIDE-acl-security (docs-project): "SSE event stream: per-type gating + audit isolation" section — per-type design, ReadQuery gate, fresh-gate membership re-derive, per-type-timing residual, rejected alternatives pointer (cacheId/mergebox/snapshot-ACL → IDEA-CQMKMD); `_events` removed from "What still leaks"; threat-model summary → "all read channels (incl. SSE) tight, MCP transport the remaining gap"
- [x] docs/acl-security.md regenerated via `just docs`

## External Docs

- [x] ~~docs/data-entry.md / metamodel.md / cli-reference.md / README.md~~ (N/A: no metamodel/CLI surface change; the SSE payload shrinks and the SPA invalidation behavior is ~unchanged — the frontend/CLAUDE.md SSE note still accurately says "invalidateAll on entity changes")
