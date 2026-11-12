---
id: RR-SP9FPDR
type: review-response
title: 'A failing join when: is only logged'
finding: 'A join whose when: cannot be evaluated waits silently; Explain shows no reason.'
severity: minor
reason: 'The run is never stuck: the last arrival continues it, and since TKT-FWQZGAM a partly arrived when: join is tested again every 5 minutes. Showing the failure needs an Explanation field and UI; moved to TKT-YNQPHPP.'
status: deferred
---
