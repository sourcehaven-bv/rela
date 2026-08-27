---
id: RR-FGHLVX
type: review-response
title: Activity bar measured router state, not data readiness — never appeared on prev/next
finding: |-
    Found by the user driving the running demo: stepping prev/next on entity detail showed no top bar at all.

    Measured in the browser rather than reasoned about: the route settles in ~99ms while the entity's own fetch lands at ~2100ms. Prev/next is a same-route param change, so the component is already mounted; `afterEach` fires almost immediately and the 2-second wait is entirely the component's own data fetch, which happens AFTER navigation has completed. A bar wired to router state is therefore correct by its own definition and useless in practice — it covers the 99ms nobody notices and misses the 2s they do.

    A DOM MutationObserver on the bar's class recorded zero mutations across a full step, confirming `isNavigating` never went true for long enough to matter.

    This also exposed an error in my own design reasoning. Ticket edge rule 2 said 'the bar is for NEW content, not refreshed content', which I had implemented as 'do not report when previous content is held'. That is backwards: keeping the old entity on screen is exactly what makes a click read as IGNORED, because nothing anywhere indicates work is happening. Stale content plus no indicator is the failure mode.
severity: significant
resolution: |-
    Added a reference-counted route-load registry to `useNavigationPending`: `beginRouteLoad()` returns a settle function, and `isNavigating` is now true while EITHER a route is in flight OR a view is still assembling its data. EntityDetail reports every load through it.

    Reporting on every load, not just cold ones, is the corrected design: a bar is the right shape here precisely BECAUSE it is peripheral and additive. It does not contradict the content the way a spinner replacing the article would — it says 'what you are looking at is being replaced', which is the missing information.

    Reference-counted rather than a flag because nested regions (an entity page and its documents panel) load concurrently and must not clear each other's contribution — this is the concurrent in-flight case where counting IS correct, as distinct from navigation itself where identity-tracking is required (RR-B7U3I8).

    The settle function is idempotent: a caller settling in both a `finally` and an error path would otherwise drive the count negative and pin the bar on permanently for every subsequent load — the same leak class as RR-B7U3I8, guarded by its own test.

    Verified in-browser after the fix: `everVisible: true`, first visible at 1478ms. Three unit tests added (settle-after-route-settled, concurrent counting, idempotent double-settle).
status: addressed
---
