---
id: RR-TRV05
type: review-response
title: "GateTraversal aliased the caller's Props slice"
severity: minor
status: addressed
finding: |-
    [security] match.Props was assigned hop.Props directly, then appended to.
    append may write into the caller's spare capacity, so a TraversalHop
    reused across two gate calls would have one call's ACL predicates
    overwritten by the next — leaving the WRONG principal's predicate
    standing. Currently latent: readQuery never populates Query.Props, so the
    append is a no-op today.
resolution: |-
    The gate now copies hop.Props into a fresh slice before folding. Pinned
    structurally (distinct backing array) rather than by observing a
    mutation, so the guarantee holds before the latent bug becomes
    reachable. Verified the test fails on the aliasing version.
---
