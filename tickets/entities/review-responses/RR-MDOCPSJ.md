---
id: RR-MDOCPSJ
type: review-response
title: "The SSE stream hid an ended session"
finding: "An EventSource reports no status, so an ended session looked like a dropped connection and reconnected forever."
severity: significant
resolution: "Addressed: a stream error also runs the session probe. Pinned by useEvents.test.ts."
status: addressed
---
