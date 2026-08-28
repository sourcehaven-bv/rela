---
id: RR-QZORXX
type: review-response
title: EmptyCardsIsAlways200 built its gate context from a different app's ACL
finding: |-
    TestDashboardPermission_EmptyCardsIsAlways200 built `d` from a throwaway newTestAppV1(t).store, then passed gateCtxFor(..., d) to a DIFFERENT app whose app.acl was a separately-constructed Declarative. It works only because both happen to use gatedNavPolicy(), so bob genuinely lacks admin:read either way — the reviewer confirmed the subtest still fails when the filter is no-op'd, so it is not vacuous today. But it passes by coincidence rather than by design, and would silently stop testing what it claims if either policy were changed independently.

    Also flagged: a two-word orphan line left mid-sentence in the permitsGatedUIElement godoc by the reflow ('It would also hide / them from EVERYONE,').
severity: nit
resolution: |-
    Moved the Declarative construction inside the subtest so ONE `d` serves both the handler's app.acl and the gate on the context, with a comment naming why: building the gate from a second policy would make the 'every card filtered' case pass by coincidence rather than because the principal genuinely lacks the permission.

    The godoc orphan line was rewrapped.
status: addressed
---
