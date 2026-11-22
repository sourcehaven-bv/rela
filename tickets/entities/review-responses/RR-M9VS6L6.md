---
id: RR-M9VS6L6
type: review-response
title: 'The wait-for-schema test did not test the order'
finding: 'The mocked load set flowsEnabled synchronously, so the test passed even if listFlows ran before the schema settled.'
severity: minor
resolution: 'The test now starts the read with loaded false, flushes, asserts listFlows was not called, then marks the schema loaded and asserts one call.'
status: addressed
---
