---
id: RR-LRG35M
type: review-response
title: Bleve admitted search ran one family query per hit
finding: resolveAdmittedHits called familyCandidates per distinct hit id, including in the trivial world, so every ACL-gated search on the default build cost O(hits) index queries.
severity: significant
resolution: The trivial world admits the matched implicit faces directly (type comes from the hit's stored fields). Other worlds read all families in one batched disjunction query per 256 ids. Pinned by TestSearchAdmitted_QueryBudgetIsHitIndependent (same count at 10 and 50 hits).
status: addressed
---
