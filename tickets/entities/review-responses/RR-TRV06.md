---
id: RR-TRV06
type: review-response
title: 'EndpointMatch docstring asserted a depth bound that did not exist'
severity: minor
status: addressed
finding: |-
    store.RelationPredicate.EndpointMatch claimed 'the naive backend
    additionally bounds its own recursion (see graphquerynaive.DepthCap), so a
    hand-built query cannot recurse without limit either'. DepthCap bounds
    expandSet's BFS — a different axis entirely. Nothing bounded EndpointMatch
    nesting: a 5000-deep hand-built chain was accepted, and pgstore emitted
    6 MB of SQL at depth 1000 (quadratic, since each alias grows one segment).

    A comment describing an intended guard that was never written is worse
    than no comment: the next reader relies on it.
resolution: |-
    The bound now exists (CheckEndpointShape) and the docstring was rewritten
    to describe what is actually enforced, including WHY it must be a refusal
    rather than a truncation. Pinned by a conformance case.
---
