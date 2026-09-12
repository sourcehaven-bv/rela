---
id: RR-VW1FAC
type: review-response
title: Frontier source gate resolved rows with a default-world header scan, so faced entities were dropped even under permit-all
finding: |-
    readableViewIDs resolved ids to types with store.ListEntityHeaders carrying NO World scope, while loadViewEntities queries with World: w.scope. Since a type declaring `faces:` stores no row at the zero coordinate (BUG-HC6I2T), the default-world scan returns nothing for a faced entity, so its type was unknown and the fail-closed branch dropped it from the frontier.

    Two consequences. (1) Functional regression: under a non-default world a recursive traversal terminated at the first faced node for EVERY principal, including a full-read one — a gate decision laundered from what was really a type-resolution failure. (2) The gate was deciding about a different graph than the one being walked, which is the shape that flips from fail-closed to fail-open on the next refactor.

    Verified empirically before fixing: with the seeded fixture, a default-world header scan for [NOTE-1, POL-1] returned only NOTE-1/note, and readableViewIDs under NopACL (permit-all) returned [NOTE-1] — the faced POL-1 dropped with every probe permitting.
severity: critical
status: addressed
resolution: |-
    readableViewIDs now takes the viewWorld and queries with the same World scope loadViewEntities uses, so the gate resolves exactly the rows the loader will. A denied world short-circuits to "nothing is expandable".

    Pinned by TestACLViewTraversal_SourceGateMatchesLoaderResolution, which asserts gate and loader agree on the same id set under both the default and a `published` world. Mutation-checked: dropping `World: w.scope` from the gate's query makes it fail with gate=[NOTE-1] loader=[POL-PUB] — total disagreement.

    Note the original report described this as a leak; it is more precisely a fail-CLOSED regression. It is recorded as critical anyway because it made the NopACL byte-identical invariant false on faced projects and because fixing it without RR-VW2FAC would have opened a real face-scoped leak.
---

## Context

Found by code review of the BUG-9Z20WH fix, against the refactored `viewsHandler`.
Coupled with RR-VW2FAC — the two had to be fixed together, since fixing this one
alone makes faced rows reachable by the gate and exposes the missing face check.
