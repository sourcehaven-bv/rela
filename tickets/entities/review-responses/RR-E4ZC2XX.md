---
id: RR-E4ZC2XX
type: review-response
title: usePageStatus fetches twice on navigation
finding: A route.path watcher refetched on every step change.
severity: nit
resolution: Dropped the path watcher. The query key and entity:changed drive refetches.
status: addressed
---
