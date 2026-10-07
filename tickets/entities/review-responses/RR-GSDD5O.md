---
id: RR-GSDD5O
type: review-response
title: Docs overstated where 422 is returned and where writes are atomic
finding: The docs claimed 422 on every path, atomic replace on the memory and file stores, and full-linkage re-point only.
severity: significant
resolution: 'Fixed in 71d1088cc. GUIDE-concepts now states the exact behaviour: 422 cardinality_exceeded on an entity PATCH, a POST of one relation and a relation restore. It states which stores roll back and that deltas also re-point.'
status: addressed
---

Review finding R1-5.
