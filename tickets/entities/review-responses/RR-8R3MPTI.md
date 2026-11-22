---
id: RR-8R3MPTI
type: review-response
title: "Fixed three-second stop for the scroll loop"
finding: "A page that settles slowly can still push the target away after three seconds."
severity: minor
resolution: "Addressed: The loop now runs at least timeoutMs and then stops after idleMs without movement, with an 8 second cap. Pinned in keepInView.test.ts."
status: addressed
---
