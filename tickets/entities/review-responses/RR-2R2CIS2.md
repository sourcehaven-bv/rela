---
id: RR-2R2CIS2
type: review-response
title: No pgstore test pins 'match the prime, not the family'
finding: TestSearch_MatchesThePrimeNotTheFamily runs only against LinearSearch; DeniedFaceIsNeverMatched passes even under a wrong variant because visCond removes the draft. A wrong (id, face) IN m inside DISTINCT ON would make a matching draft the prime.
severity: significant
resolution: 'Plan changed: storetest case in a non-default world without face ACL where the term exists only on a non-prime face, for gated and ungated search, under and over the threshold.'
status: addressed
---
