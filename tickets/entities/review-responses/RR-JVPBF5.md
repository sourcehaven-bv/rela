---
id: RR-JVPBF5
type: review-response
title: A blocked frame write delays the expiry
finding: The expiry case is one more case in the same select as every other case. A frame write can block for up to sseMissedPings * ssePingInterval (2 x 25 s = 50 s) before its write deadline fails. While it blocks, the loop checks neither the timer nor ctx.Done. So the stream, and the liveConn the hub keeps replaying as the owner, can outlive exp by about 50 s. No frame reaches the client in that window because the write is stuck, but replays keep running as the expired owner. State this bound in the plan and in the runSSELoop comment, so nobody later claims the stream ends exactly at exp. Optionally, set the write deadline to min(now+deadline, exp) so that a stuck write also fails at the expiry.
severity: minor
resolution: 'Plan updated: sseConn caps each write deadline at the expiry, so a blocked write fails at the expiry and the loop returns.'
status: addressed
---
