---
id: RR-R5I7UE0
type: review-response
title: Active state listed by state is a second ungated read path
finding: List by States feeds an inbox from the ungated store while the _flow-run header carries the same fact behind the row ACL.
severity: significant
resolution: States dropped from Query and the (flow, state) index removed; the run store is authoritative and the inbox reads headers through the visibility gate (TKT-9IT3LN8).
status: addressed
---
