---
id: RR-Z9VS5V
type: review-response
title: Storage logic duplicates useListGrouping
finding: Key prefix, defensive parse and quota catch are reimplemented from useListGrouping.
severity: minor
reason: The two stores have different shapes (a set of closed group ids versus a map of overrides against a config default), so a shared helper would only share the try/catch around JSON.parse. Extracting it means changing useListGrouping, which is outside this ticket; worth doing when a third caller appears.
status: deferred
---
