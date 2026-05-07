---
id: RR-P7E24
type: review-response
title: SSE-driven refresh has no anchor in current code; dirty registry solves nonexistent problem unless we also wire up entity refetch
finding: |-
    Plan section 4 says: "SSE handler consults [the dirty registry] before invalidating ... refetch the entity but skip overwriting formData for dirty fields." But useEvents.ts:130-140 calls entitiesStore.invalidateAll() — a global cache wipe; no per-entity refetch happens. DynamicForm's loadEntity() runs only on mount, never as a reaction to cache invalidation. Today the form ignores SSE entirely. So the registry-based 'skip overwriting dirty fields on refetch' mechanism solves a problem that doesn't currently exist in the form. The actual problem is: auto-save needs to introduce SSE-driven refresh of formData (so multi-tab edits show up), and *that's* what introduces the clobber risk the registry guards against.

    The plan must specify (a) a reactive trigger that re-pulls server state into formData after entity:updated, (b) merge semantics (server wins for non-dirty, local wins for dirty), and (c) what happens when a dirty field also changed on the server.
severity: critical
resolution: 'Plan now adds an explicit SSE refresh hook in DynamicForm (subscribes to entity:updated for own entity, refetches via entitiesStore.fetchEntity, merges via useAutoSave.mergeServerResponse). Merge rule: server wins for non-dirty properties, local wins for dirty; conflicts on dirty fields silently keep local with documented last-write-wins on next save. AC #9 covers.'
status: addressed
---
