---
id: RR-JJ67NA
type: review-response
title: 'Stall tests depend on the 60s pending-job poll interval'
finding: 'The 5s and 10s waits only detect a dead listener because the poll interval is 60s; lowering it would make the tests pass on broken code.'
severity: significant
resolution: 'Added comments at the waits stating they must stay below the pending-job poll interval. Neither repo overrides the 60s default.'
status: addressed
---
