---
id: RR-X0YJAF
type: review-response
title: 'PR-C: outage rendered as an empty world, duplicate ?world= resolved silently, ordering pinned only by source scan'
finding: |-
    Three further PR-C findings, all verified:

    1. getWorldEntity returned real store errors and getVisible folded them into not-found -> 404. On the WORLD path the underlying read is a ListEntities iterator whose error is ALWAYS infrastructure (a genuine miss is errWorldEntityAbsent), so a backend outage under ?world=published rendered as 'this entity has no published face'. That is the exact mistake resolveWorld refuses to make one file over (RR-4TFZNL) — made in the opposite direction, in the same PR.

    2. `?world=` used Query().Get(), which takes the FIRST value. So `?world=default&world=published` served the default world under a request that also asked for published — a client-side param-append bug becoming a silent wrong-face serve.

    3. TestGrantCheckPrecedesResolverConstruction asserted SOURCE order only. Its premise (last wrap = outermost) is correct, but it is fooled by moving either call into a helper, and nothing in the suite exercised attachWorld through the real router at all.
severity: significant
resolution: |-
    1. getVisible now propagates any non-errWorldEntityAbsent error on the world path (5xx), while the default-world branch keeps GetEntity's inherited miss-is-not-found contract unchanged.

    2. More than one ?world= is now a 400 (`duplicate_world`) rather than resolved by a precedence rule nobody would remember. Pinned by TestAttachWorld_DuplicateParamRejected.

    3. Added TestWorldGrantCheckThroughTheRealRouter — a real Declarative ACL denying the world, driven through app.NewRouter(). Getting it to DISCRIMINATE took a second pass: the stub world initially excluded everything, so 'denied' and 'permitted' both rendered empty and the test passed under reversed ordering. The stub now resolves to the default state, so the two outcomes genuinely differ. Verified both guards now fail when the middleware order is reversed.
status: addressed
---
