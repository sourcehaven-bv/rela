---
id: RR-TRV04
type: review-response
title: 'Nested inheritance expansion honored by Go backend, dropped by pgstore'
severity: significant
status: addressed
finding: |-
    [security] nestedPredicateSQL never emitted InheritThrough or
    EntityInheritThrough, while graphquerynaive routed the same predicate back
    through matchesPredicate and DID expand both closures. GateTraversal folds
    the ACL read query into the hop, and that query carries
    EntityInheritThrough whenever the policy declares inherit_roles_through —
    so the same policy and principal were gated differently on postgres than
    on fs/memory/sqlite, with no test looking there.

    The docstring claimed 'a caller must not set them here', contradicted by
    the only caller in the tree.
resolution: |-
    Both backends now REFUSE a nested hop carrying either expansion, via the
    shared CheckEndpointShape — one answer the store contract can state.
    GateTraversal additionally refuses a CHAINED hop whose folded read query
    carries them, so the failure is a clean denial rather than a store error.
    Pinned by a conformance case that failed on the Go backend when added.
---
