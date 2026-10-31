---
id: RR-3PEFVQP
type: review-response
title: Use a real router in the property tests
finding: Existing EntityDetail tests mock vue-router. The route-change watcher is the subject of this test, so a mock would hide it.
severity: minor
resolution: Property tests mount with createRouter and createMemoryHistory and switch entities through the panel props and router.push.
status: addressed
---
