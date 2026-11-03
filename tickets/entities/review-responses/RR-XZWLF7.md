---
id: RR-XZWLF7
type: review-response
title: New target's baseline is lost on timeout
finding: 'Architecture design review: a target missing from the cursor was baselined in memory, but a timeout returned the old cursor, so the target was never baselined across calls and a change between calls was lost.'
severity: significant
resolution: Every return carries a cursor built from the latest finished digests; unfinished targets keep their previous digest. AC4 now covers adding a target, timing out, changing it, and calling again.
status: addressed
---
