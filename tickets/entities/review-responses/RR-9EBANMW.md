---
id: RR-9EBANMW
type: review-response
title: "No App-level test of the start-up order"
finding: "No test mounts App.vue to check that a suspended start loads nothing and that main.ts mounts after the status read or its timeout."
severity: minor
reason: "App.vue and main.ts need the router, stores and the whole shell; the status handling and the screens are unit-tested, and the request count was checked by hand through a proxy that reports a suspended workspace."
status: deferred
---
