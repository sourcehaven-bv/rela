---
id: RR-TNH89J
type: review-response
title: No similarity floor for phase 3 candidates
finding: Context terms alone (~0.485) plus any small similarity clear the 0.5 floor; phase 2 has fuzzyMinSimilarity 0.6 but the plan names none, so AC5 could fail.
severity: critical
resolution: 'Plan revised: overall length-weighted similarity and each endpoint must be >= 0.6 (fuzzyMinSimilarity), else discarded. Negative test: quote rewritten with prefix/suffix intact orphans.'
status: addressed
---
