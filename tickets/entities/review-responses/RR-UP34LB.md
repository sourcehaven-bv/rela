---
id: RR-UP34LB
type: review-response
title: Failed select clears the record without a live() check
finding: useSessionDecisions.ts:444 clears formDecision when select returns false, even if the click is no longer live. After a channel switch (resetDecisions sets deciding false), a click in the new channel could record a compose decision that this late clear then removes. Guard the clear with live().
severity: nit
resolution: "addressed\tWhen select fails, the record is cleared only while the click is still live."
status: addressed
---
