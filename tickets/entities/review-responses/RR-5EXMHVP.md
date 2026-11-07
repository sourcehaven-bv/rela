---
id: RR-5EXMHVP
type: review-response
title: 'Security: step 3 must not serve stored hashes over hidden fields'
finding: No finding in this change (no read surface). The stored element covers hidden fields, so serving its hash, limbs or range sums to a reader with a hidden field is a value oracle. Step 3 must gate rows before summing or counting, gate both relation endpoints, use a per-row SQL predicate like SearchFieldScope for rows with hidden fields, and never cache range sums across principals.
severity: significant
resolution: 'TKT-IQU106C enforces all four rules inside each backend''s range query: the row gate is in the WHERE clause under the aggregate; a relation needs both endpoint EXISTS checks under the scope; a stored hash is folded or listed only when a per-row predicate (COALESCEd to false) shows no hidden field holds a value, otherwise the row is pending; nothing is cached. The conformance suite, a 300-case differential against the Go reference and an archguard caller allowlist pin it.'
reason: Nothing in this ticket reads the columns out. The constraints belong to TKT-IQU106C, which adds the range-fingerprint read; recorded there as its design input.
status: addressed
---
