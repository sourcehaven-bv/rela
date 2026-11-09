---
id: RR-S0AWFF1
type: review-response
title: KeptEntityError is swallowed silently
finding: A create output whose follow-up write failed was counted as done without a log line.
severity: minor
resolution: It is now logged at warn with the entity id and the error.
status: addressed
---
