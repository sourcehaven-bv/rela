---
id: RR-WU1TVBI
type: review-response
title: 'No test for a failed schema load'
finding: 'The tests did not cover load() rejecting.'
severity: minor
resolution: 'Not applicable after the first fix: useFlowList no longer calls load(), so it has no failure path of its own; it waits for App.vue''s load.'
status: addressed
---
