---
id: RR-6G1K
type: review-response
title: meta and automation published non-atomically; readers observe inconsistent pair
finding: 'Between w.meta.Store(newMeta) and w.automation.Swap(...) a concurrent CreateEntity sees the new metamodel but the old automation engine (built from old metamodel). The struct comment claims ''no caller reads these fields as a coherent set'' but CreateEntity reads automation.Load() and then w.Meta() inside createEntityCore — that IS a coherent-set read. Fix: bundle meta+automation+searchIdx into a single workspaceState struct held as atomic.Pointer, and publish once per reload.'
severity: significant
resolution: Bundled meta, automation, and searchIdx into an immutable workspaceState struct held via atomic.Pointer[workspaceState]. Reload builds a single new state and publishes it once. Every reader (CreateEntity, UpdateEntity, runCreatedEntityAutomation, indexEntity, removeFromIndex, Meta, Search) now calls w.state.Load() once and works against that coherent snapshot. This is the L1 recommendation from the cranky review.
status: addressed
---
