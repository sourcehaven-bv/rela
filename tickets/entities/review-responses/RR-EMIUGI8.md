---
id: RR-EMIUGI8
type: review-response
title: Decide whether a run's own outputs count as change events
finding: A change branch without roles admits any write, including the flow's own set output.
severity: minor
reason: 'Decided in TKT-9IT3LN8: a run''s own outputs do not count as change events. M1d has no change events; TKT-FWQZGAM implements the rule in the write-path hook.'
status: deferred
---
