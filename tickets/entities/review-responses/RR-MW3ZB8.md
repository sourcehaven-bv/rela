---
id: RR-MW3ZB8
type: review-response
title: naive MatchingIDs ranks before Any
finding: graphquerynaive.MatchingIDs listed primes then tested Any, contradicting its own Run and pgstore.
severity: significant
resolution: MatchingIDs iterates collectByType, the same candidates Run ranks; covered by the Any subtest on every backend.
status: addressed
---
