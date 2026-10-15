---
id: RR-3WS1KB
type: review-response
title: No test for the load generation guard
finding: The generation guard in DashboardView that drops stale loads had no test.
severity: minor
resolution: Added a Vitest case where an older load resolves after a newer one and the newer title stays.
status: addressed
---
