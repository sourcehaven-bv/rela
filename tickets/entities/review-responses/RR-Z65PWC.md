---
id: RR-Z65PWC
type: review-response
title: Ordered-subsequence invariant only holds per-backend; parity baseline undefined
finding: Bleve and trgm are different rankers; pgstore's visible order cannot be asserted a subsequence of bleve's ungated order. The invariant is only valid within one backend (gated-of-backend-X ⊆ ungated-of-backend-X). Conformance case 1 'parity with ungated Search' must define which ungated query (capped vs uncapped) is the baseline.
severity: significant
resolution: 'Plan rev 2: invariants restated per-backend (gated-of-X ⊆ ungated-of-X); parity baseline defined as same backend + same Query incl. limit; fixture corpora stay below all candidate bounds so truncation never confounds the subsequence check (truncation pinned separately by case 5).'
status: addressed
---
