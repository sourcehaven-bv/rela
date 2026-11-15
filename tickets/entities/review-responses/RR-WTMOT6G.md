---
id: RR-WTMOT6G
type: review-response
title: No short-circuit on hub entities
finding: Related lists every stored edge and matches all far ids to answer an any-question; a hub costs one full list and one large match inside the write lock.
severity: minor
reason: Added edges are now asked first. Chunked or store-level existence queries are a follow-up; the cost is bounded by one list and one MatchingIDs per hop.
status: deferred
---
