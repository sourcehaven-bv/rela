---
id: RR-ZS04NY
type: review-response
title: 'Minor cleanups: stale handleSSE godoc, dead entityIds in Vue, zero-value frame guard, stale dirtyFormRegistry comment'
finding: 'Code review M1-M4: (M1) handleSSE godoc still documented the removed entity:created/updated/deleted {type,id} events; (M2) entityIds ref was write-only dead state in DocumentView.vue + DocumentsPanel.vue after the id-match was removed; (M3) a zero-value sseEvent would hit the default case and write a malformed empty frame; (M4) dirtyFormRegistry.ts comment named the dead entity:updated event.'
severity: nit
resolution: 'M1: handleSSE godoc rewritten to describe entity:changed {type}-only. M2: entityIds ref + assignments removed from both Vue components (and the wasted result.entity_ids read). M3: runSSELoop now has an explicit Name!='''' case for non-entity frames and a default that slog.Warns + drops a zero-value frame instead of emitting a malformed one. M4: dirtyFormRegistry comment updated to ''SSE-triggered re-fetch (entity:changed)''. Frontend typecheck + 1032 tests green.'
status: addressed
---
