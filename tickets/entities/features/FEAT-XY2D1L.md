---
id: FEAT-XY2D1L
type: feature
title: Stale-while-revalidate query cache for the data-entry SPA (Pinia Colada)
description: 'Replace the hand-rolled entities-store TTL cache + invalidateAll() SSE strategy with Pinia Colada queries: views subscribe to query keys, SSE events invalidate targeted keys ([entity, type, id] / [list, type]), invalidation marks queries stale and triggers background refetch (no spinner flash, no lost local copies), mutations update the cache through optimistic copy-on-write with rollback. Migrated view-by-view; when the last consumer moves over, the entities store TTL cache, cacheVersion, and the four bespoke cancellation schemes are deleted. Motivated by the 2026-06 frontend review (sections B2/A4: stale-response race, invalidateAll discarding event granularity, only KanbanView reacting to SSE, five independent local copies of entity state).'
status: in-progress
---

Chosen over TanStack Vue Query for the Vue/Pinia-native fit and smaller surface;
over a hand-rolled SWR core because the codebase's bespoke-infrastructure track
record argued against it (four divergent cancellation schemes, half-built etag
and dirty-registry mechanisms). Pinia Colada is post-1.0 (1.3.x as of 2026-05)
and its API is close enough to TanStack Query that a later migration would be
mechanical.

Migration order: KanbanView (proof of pattern — suffers most visibly from
invalidateAll), then EntityList, EntityDetail, Dashboard/Search/Analyze, then
delete the entities-store cache.
