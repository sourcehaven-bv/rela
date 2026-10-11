---
id: RR-VZ2MX2
type: review-response
title: destinationHolds treated any read error as an empty destination
finding: A transient GetEntity error reported no collision, so the pre-check let a thread merge into an occupied destination.
severity: significant
resolution: Only store.ErrNotFound means absent; any other error is returned with the id and face named.
status: addressed
---
