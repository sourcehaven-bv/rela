---
id: RR-NAOSUDY
type: review-response
title: Webhook side of the race untested
finding: Only the lazy path was tested; the webhook now gets the finished run and a 200.
severity: minor
resolution: TestProvision_NoRerunAfterSuccess is a table over the lazy request (202) and the webhook (503 with Retry-After).
status: addressed
---
