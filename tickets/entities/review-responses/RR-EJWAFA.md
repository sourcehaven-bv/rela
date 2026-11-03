---
id: RR-EJWAFA
type: review-response
title: No test at the api/piles.ts URL layer
finding: The bug was the ?world= in the request URL, but the composable test mocks api/piles and only counts arguments.
severity: minor
resolution: 'Added frontend/src/api/piles.test.ts: reads carry the world, the four writes do not. Checked to fail against the old module.'
status: addressed
---
