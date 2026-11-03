---
id: RR-6E7TIU
type: review-response
title: Prime staleness within a request not tested
finding: No test shows that a prime taken before a write is not reused after it.
severity: nit
resolution: TestWithoutLocalRolePrime_SeesLaterEdges covers it.
status: addressed
---
