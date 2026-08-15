---
id: RR-DBTEST2
type: review-response
title: The table-rows test asserted sort order, so it pinned nothing this ticket changed
finding: |-
    'sorts table rows once even though the template reads them twice' asserted only `cells === ['a','b','c']`. The pre-TKT-ERHWL0 code sorted correctly too, so the assertion held equally against the implementation the ticket replaces.

    Demonstrated: reverting to the non-memoized getters failed the breakdown test (6 reads vs 3) while this one PASSED. Its name promises 'once' and 'twice' and it verified neither — it pinned sorting, which was never at risk.
severity: significant
resolution: |-
    Instrumented the same way as the breakdown test: a counting getter on `title`, so the assertion measures how many times the rows were derived rather than what order they came out in. The order assertion is kept — it is still worth pinning — but it is no longer the only one.

    The bound is expressed as `expect(reads).toBeLessThan(15)` with `15` named as the measured pre-change count, rather than a bare magic number: sort comparisons plus one read per rendered cell is implementation detail that could shift, while 'fewer than deriving twice' is the property under test.

    Mutation-verified: with the template calling the derivations twice again, it fails at 21 reads.
status: addressed
