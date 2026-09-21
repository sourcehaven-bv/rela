---
id: RR-N8M7F4
type: review-response
title: 'Deferred: property-based tests, the shared ranker prologue, and the MAX_RESULTS slice-order comment'
finding: 'Three suggestions from review: (1) give the rankers fast-check property tests for the never-widens and subset-of-input invariants, which are currently asserted over seven hand-picked queries; (2) the two rankers share a near-duplicate guard prologue with subtly different conditions, and the explanatory comment lives on only one of them; (3) `rankEntities(...).slice(0, MAX_RESULTS)` slices AFTER the ID-tier sort, which is load-bearing (slicing first would drop a 21st-position exact ID match before the tier could promote it) but was undocumented and untested.'
severity: nit
resolution: '(3) done: added a test asserting an exact ID match at the tail of a 31-row response survives into the top rank, with a comment naming the ordering as load-bearing. (2) partially done: `tokenizerSawWholeNeedle` now factors out the guard both rankers share, and `rankTypeNames` carries a pointer to the full explanation rather than a bare duplicate line. (1) not done, and deliberately: `fast-check` is not currently a dependency of this project, and adding one to assert invariants already covered by a 7-case matrix plus the new mixed-script matrix is not worth a new dependency in this ticket. Worth doing if a future change swaps the scorer.'
status: addressed
---
