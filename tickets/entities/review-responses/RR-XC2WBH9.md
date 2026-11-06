---
id: RR-XC2WBH9
type: review-response
title: Engine limits duplicated in flow and realtimelimits
finding: flow.MaxStates, MaxTimers and MinTimer duplicate the realtimelimits session caps.
severity: nit
reason: The runtime ticket TKT-9IT3LN8 moves the session engine callers onto the flow constants; realtimelimits keeps channel-specific caps.
status: deferred
---
