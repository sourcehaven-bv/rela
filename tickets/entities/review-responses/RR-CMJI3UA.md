---
id: RR-CMJI3UA
type: review-response
title: related returned an empty table for an unreadable source
finding: An actor that cannot read the source got an empty table rather than nil as get returns.
severity: minor
resolution: related resolves the source first and returns nil on not-found; TestRelatedReadsOutgoingTargets checks an outsider and a missing id.
status: addressed
---
