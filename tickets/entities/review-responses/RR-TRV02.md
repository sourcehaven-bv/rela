---
id: RR-TRV02
type: review-response
title: 'Client attenuation ceiling not applied to the field gate'
severity: significant
status: addressed
finding: |-
    [security] GateTraversal consulted only the policy-wide
    Policy.ConditionallyVisible, which reads raw un-attenuated p.Roles. It
    never called Request.FieldCeilingFor, the compiled per-principal ceiling
    carrying redact:/visible: from client_baselines and scope_grants.

    A client attenuated by `redact: {person: [salary]}` acting as a user who
    holds salary unconditionally would pass the policy-wide check and the row
    gate, then recover the value by inference. The traversal filter reached
    strictly further than a plain read for the same principal — the ceiling
    is supposed to only ever narrow.
resolution: |-
    GateTraversal now consults FieldCeilingFor(hop.EntityType) in addition to
    the policy-wide check, refusing any property the ceiling redacts or
    withholds via its closed world. Both checks are kept: policy-wide stays
    load-time and principal-independent; the ceiling one is necessarily
    per-principal. Pinned by a test that fails when the check is removed.
---
