---
id: RR-JQFF1II
type: review-response
title: componentScope.run did nothing
finding: Vue 3.5 computeds do not register with the active effect scope, so the scope plumbing and its comment were misleading.
severity: nit
resolution: Removed the scope plumbing; the comment now states that the composable registers no watchers or hooks.
status: addressed
---
