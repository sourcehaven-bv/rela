---
id: RR-TRV03
type: review-response
title: 'Depth bound became a row-gate bypass under an outer Negate'
severity: critical
status: addressed
finding: |-
    The nesting bound initially rendered a too-deep arm as `AND FALSE`. That
    is sound only under a POSITIVE exists. Nested inside a negated predicate
    it inverts to `NOT EXISTS(... AND FALSE)`, which is TRUE for every row of
    the type — so the guard against a pathological query became a row-gate
    bypass that matched everything. Verified by emitting SQL for a 9-deep
    chain with Negate on the outermost hop.

    The naive backend meanwhile ERRORED on the same input, so the two
    backends answered 'error' vs 'everything'.
resolution: |-
    Replaced with an up-front REFUSAL in one shared
    graphquerynaive.CheckEndpointShape, called by pgstore's
    checkGraphQueryScope and by Run/Count/MatchingIDs. No unsatisfiable arm
    is ever emitted. Pinned by a conformance case asserting refusal, and by
    a negated-chained-hop case asserting the answer is not 'everything'.
---
