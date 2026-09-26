---
id: RR-ARJZD0
type: review-response
title: search_entities hydrated hits one by one with an unbounded limit
finding: Per-hit GetEntity through the policy reader (N+1, one ACL Request each) and limit<=0 meant no cap (bleve up to 10000).
severity: significant
resolution: Hits are hydrated with one ListEntities(IDs) read and emitted in hit order. limit is clamped to [1, 200].
status: addressed
---
