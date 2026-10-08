---
id: RR-FJXDU7
type: review-response
title: Keyboard moves lagged behind queued requests
finding: The optimistic order was applied inside the queued send, so a second arrow press showed nothing for a round trip and refocus could miss.
severity: minor
resolution: Both composables apply the optimistic order synchronously in onReorder and queue only the request. Tested by 'shows the move before the call returns'.
status: addressed
---
