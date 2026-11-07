---
id: RR-5EXMHVP
type: review-response
title: 'Security: step 3 must not serve stored hashes over hidden fields'
finding: No finding in this change (no read surface). The stored element covers hidden fields, so serving its hash, limbs or range sums to a reader with a hidden field is a value oracle. Step 3 must gate rows before summing or counting, gate both relation endpoints, use a per-row SQL predicate like SearchFieldScope for rows with hidden fields, and never cache range sums across principals.
severity: significant
reason: Nothing in this ticket reads the columns out. The constraints belong to TKT-IQU106C, which adds the range-fingerprint read; recorded there as its design input.
status: deferred
---
