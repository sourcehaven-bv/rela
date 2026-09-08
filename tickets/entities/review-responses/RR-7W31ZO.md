---
id: RR-7W31ZO
type: review-response
title: Redirect watcher depends on the schema being loaded when the view resolves
finding: EntityDetail.vue watch(worldAbsent) reads worldInfo at fire time; if the schema is still loading, target is undefined and the redirect silently never happens. Watch both sources or state the degradation.
severity: minor
resolution: The watcher observes [viewData, worldInfo], so a schema that lands after the view still triggers the redirect. Pinned by the 'redirects when the schema arrives AFTER the view' test.
status: addressed
---
