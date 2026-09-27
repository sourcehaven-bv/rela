---
id: RR-1TWZ95
type: review-response
title: '''still active'' skip logs at INFO every tick'
finding: A long for_each run or a task slower than its interval logs one INFO line per minute.
severity: minor
resolution: The skip is logged at DEBUG. A genuinely stuck run surfaces as 'run abandoned' at ERROR.
status: addressed
---
