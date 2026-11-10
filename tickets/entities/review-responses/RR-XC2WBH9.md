---
id: RR-XC2WBH9
type: review-response
title: Engine limits duplicated in flow and realtimelimits
finding: flow.MaxStates, MaxTimers and MinTimer duplicate the realtimelimits session caps.
severity: nit
resolution: 'Fixed in TKT-9IT3LN8: the session caps are gone from realtimelimits; realtimecfg and realtime use flow.MaxStates, flow.MaxTimers, flow.MinTimer and flow.MaxReminders.'
reason: The runtime ticket TKT-9IT3LN8 moves the session engine callers onto the flow constants; realtimelimits keeps channel-specific caps.
status: addressed
---
