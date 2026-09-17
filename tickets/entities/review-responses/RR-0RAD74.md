---
id: RR-0RAD74
type: review-response
title: fetchListInternal's fetch parameter type erased the AbortSignal
finding: 'fetchListInternal typed its fetch parameter as (type, params?) even though both listEntities and listAllEntities accept a third signal argument. TypeScript accepts the narrower type, so the signal vanished from the seam. Pre-existing for fetchList (one wasted request), but materially amplified by fetchAllList: an uncancellable SEQUENTIAL loop of up to 50 requests that writes into the shared entity cache on every iteration, so a superseded picker keeps populating cache keys under its own params.world after a new route has read them. listAllEntities'' own docblock anticipates exactly this.'
severity: critical
resolution: 'Threaded signal end to end: fetchList and fetchAllList both accept signal?: AbortSignal, fetchListInternal takes it and forwards it to fetch(type, params, signal), and the fetch parameter is now typed (type, params?, signal?) => Promise<ListResponse<Entity>> so the signal is visible in the type rather than silently droppable.'
status: addressed
---
