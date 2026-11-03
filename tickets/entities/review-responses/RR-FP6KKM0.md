---
id: RR-FP6KKM0
type: review-response
title: redactingSeq comment overstates that primes do not pile up
finding: 'Code review: A traversal memo already on the base ctx is shared, so each page adds to it.'
severity: minor
resolution: The godoc now says local-role primes do not pile up, a shared traversal memo grows with the rows read, as it would if the caller primed them all.
status: addressed
---
