---
id: RR-6Z5ZRC
type: review-response
title: Detail reload debounce starves under a steady event stream
finding: The debounce restarted on every event. A steady stream of events postponed the reload forever.
severity: minor
resolution: Fixed in 82f0f9a0d. The debounce has a 1 s max wait. Covered by the test that reloads during a steady stream in EntityDetail.events.test.ts.
status: addressed
---

Review finding R2-8.
