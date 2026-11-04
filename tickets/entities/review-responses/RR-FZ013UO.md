---
id: RR-FZ013UO
type: review-response
title: '[security] Served-pending timing comment contradicts the dueOnly fallback rule'
finding: '[security] live_served.go said the replay is due on the next tick whatever else changed. A topic round that overflows sends no frame for a sub it took only because it was pending and replays it later; the overflow depends on activity the subscriber may not see. A GET now makes a sub pending at will.'
severity: minor
resolution: The header comment now names the fallback exception. The class predates this change (PUT recheck pending) and needs more than liveRoundCap pairs in a topic round.
status: addressed
---
