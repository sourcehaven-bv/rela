---
id: RR-U5LKGQ
type: review-response
title: 'No test pinned the argument NewRouter passes to the warning'
finding: |-
    `TestUnmatchedReject_NewRouterWarnsWhenGateMissing` asserted only that SOME
    warning appeared. It never exercised the gate-wired path through NewRouter, so
    a helper ignoring `jwtWired` entirely still passed it -- and the discrimination
    lived only in a test calling the helper directly.

    Concretely: swapping `a.jwtGate != nil` for a literal `false` at the call site
    left the whole suite green. Nothing asserted NewRouter passes the right
    boolean.
severity: significant
resolution: |-
    Added `TestUnmatchedReject_NewRouterSilentWhenGateWired`: wires a gate via the
    existing `mustSetJWTGate` helper, builds the router, asserts silence.
    Mutation-verified -- the literal-`false` substitution now reddens it.

    Also tightened the original test to assert on the specific cause
    ("no JWT gate is wired") rather than the generic "NO effect", so the two
    NewRouter tests distinguish which branch fired.

    Same shape as the ticket itself, one level up: I had a test proving the
    composition site calls SOMETHING, and mistook it for proof that it calls it
    CORRECTLY.
status: addressed
---

## Resolution

Added `TestUnmatchedReject_NewRouterSilentWhenGateWired`: wires a gate via the
existing `mustSetJWTGate` helper, builds the router, asserts silence.
Mutation-verified -- the literal-`false` substitution now reddens it.

Also tightened the original test to assert on the specific cause
("no JWT gate is wired") rather than the generic "NO effect", so the two
NewRouter tests distinguish which branch fired.

Same shape as the ticket itself, one level up: I had a test proving the
composition site calls SOMETHING, and mistook it for proof that it calls it
CORRECTLY.
