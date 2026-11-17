---
id: RR-290OTWT
type: review-response
title: Webhook dedup set failed open on a load or save error
finding: The webhook seen-set accepted the webhook when loading or saving the set in tenant state failed, which would allow a replay.
severity: minor
resolution: The handler now answers 503 with Retry-After 30 when the seen-set cannot be loaded or saved (TestSeenSet_FailsClosed). Commit f2f2dac83.
status: addressed
---
