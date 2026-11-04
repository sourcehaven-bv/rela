---
id: RR-XNOW58C
type: review-response
title: Marker schema comment overstates the guarantee
finding: The marker frame can precede the note frame in the same flush so the server has not reported every earlier write when the marker frame arrives.
severity: nit
resolution: Comment now says the flush that reports a marker write reports or follows every earlier write.
status: addressed
---
