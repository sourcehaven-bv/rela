---
id: RR-2I3H
type: review-response
title: 'Mount race: two fetches on initial load'
finding: 'Today onMounted(loadEntities) runs once. With URL sync, if reading route.query happens after the first loadEntities is scheduled, fetch #1 has empty filters. Then either nothing re-fetches (URL filters silently never apply) or you add a watcher and fetch twice on every mount. Spec the ordering: read route.query synchronously in setup before first loadEntities call.'
severity: significant
resolution: useUrlFilterSync calls readFromQuery() synchronously in setup, before EntityList's first loadEntities runs. Specified in plan section 7.
status: addressed
---
