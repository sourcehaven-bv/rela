---
id: RR-1ONA
type: review-response
title: togglingIndices set not cleared on route navigation
finding: Per-instance Set is not cleared in the route watcher. A click on entity A in flight + navigate to B + click same data-cb-idx on B before A's PATCH resolves = silently swallowed. Tiny window; clear in the entityType/entityId watch.
severity: minor
resolution: Added `togglingIndices.clear()` at the top of the route-change watcher callback. Combined with the viewData identity check from RR-MV0H, in-flight A-toggles cannot clobber B's view AND B's checkboxes at the same data-cb-idx are immediately clickable on arrival.
status: addressed
---
