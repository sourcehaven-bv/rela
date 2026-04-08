---
id: RR-RGDB
type: review-response
title: Double-fetch / stale-response race on list switch
finding: 'When navigating between lists in EntityList.vue, the listId watcher calls loadEntities() and the new filters watcher (deep) ALSO fires when the URL sync composable re-seeds filters.value, causing two concurrent fetches with no abort/coalesce. If the first fetch resolves last, list A''s data renders under list B''s config — classic stale-response bug. Fix: pick one driver. Options: (a) remove loadEntities() from one of the two watchers, (b) introduce a single scheduleFetch() with a generation counter that drops stale responses, (c) add AbortController support to entitiesStore.fetchList.'
severity: critical
resolution: 'EntityList.vue: added fetchGeneration counter that all loadEntities() calls capture and verify before writing results; stale responses are dropped. Added scheduleFetch() wrapper that coalesces multiple synchronous triggers (list switch + filter reseed + sort + page) into a single loadEntities() call per microtask via nextTick. All watchers and handlers now call scheduleFetch() instead of loadEntities() directly, so the double-fetch on list switch can no longer happen, and if somehow it did, the stale response would be dropped.'
status: addressed
---
