---
id: RR-S75WCAW
type: review-response
title: "A failed probe could block later probes"
finding: "The probing promise was assigned after the request started and not cleared on every path."
severity: significant
resolution: "Addressed: the promise is assigned before it runs and cleared in finally."
status: addressed
---
