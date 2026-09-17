---
id: RR-ZPPRIH
type: review-response
title: 'No request sequencing: a superseded render could paint under the wrong document'
finding: loadDocument had no generation guard, so two in-flight renders could complete out of order. A slow warm SSE render (reliably slow because refresh=true bypasses the server's render cache) could land after a fast cold document switch and paint document A's body under document B's title. Removing the unconditional blank took away the thing that used to make the race visible.
severity: critical
resolution: 'Added a monotonic renderGeneration counter to both DocumentView and DocumentsPanel. A response may only touch state if no newer render has started since: the success path, the error path (a superseded failure raises no toast) and the finally block (an older render may not clear loading) are each fenced. Pinned by three tests; removing the fence fails two of them.'
status: addressed
---
