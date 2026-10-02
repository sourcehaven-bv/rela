---
id: RR-O19JKO
type: review-response
title: Handler timeout fires with neoq JobTimeout
finding: The handler context deadline equals neoq's JobTimeout, so neoq can abandon the job before the handler returns its own error.
severity: minor
resolution: dispatch cancels handlerCancelGrace (30s) before handlerTimeout.
status: addressed
---
