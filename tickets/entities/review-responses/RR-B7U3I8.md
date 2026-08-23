---
id: RR-B7U3I8
type: review-response
title: 'ActivityBar counter leaks on cancelled navigation: onError early-returns before any decrement'
finding: |-
    The plan says the navigation counter is "incremented in router.beforeEach and decremented in afterEach+onError". That is unsafe as written.

    router/index.ts:240-248 shows onError EARLY-RETURNS for exactly the cases where afterEach does not fire:

        if (isNavigationFailure(err, NavigationFailureType.cancelled) || ... aborted || ... duplicated) return
        if (isCancelledFetch(err)) return
        if (err === undefined || err === null) return

    So for a cancelled/aborted/duplicated navigation — which BUG-6C3V documents as ROUTINE in Firefox during rapid navigation, and which the 15 lazy `() => import()` routes make common — the counter is incremented and never decremented. The bar then displays forever, on every subsequent page, until reload. This is worse than the flashing the ticket set out to fix.

    A naive fix (decrementing at the top of onError) is also wrong: onError does not fire at all for a plain guard-cancelled navigation, and it can fire for errors that were never counted.

    Required design change: do not pair beforeEach with afterEach/onError at all. Vue Router guarantees afterEach runs for SUCCESSFUL navigations only, so any counter keyed on beforeEach is structurally leak-prone.

    Use one of:
    (a) Track the pending navigation by identity, not by count: store the target route in beforeEach and clear it in afterEach; a NEW beforeEach overwrites the previous target, so a superseded navigation cannot leave a residue. Overlapping navigations then collapse to "the latest one", which is also the correct UX (edge rule 3's reference counting is about concurrent independent fetches, not about navigations, which supersede each other).
    (b) Keep a counter but decrement it from a `watch` on `route.fullPath` plus a safety timeout, so a lost navigation self-heals.

    (a) is preferred: it is simpler and cannot leak by construction. Note this WEAKENS edge rule 3 for the navigation case specifically — reference counting still applies to the query-count half of the global signal, if that is included.
severity: critical
resolution: 'Plan Approach §2 rewritten. The beforeEach-increment/afterEach-decrement counter is replaced with identity-tracking: a single shallowRef holding the pending target route, overwritten by each beforeEach and cleared in afterEach. A superseded navigation overwrites rather than accumulating, so the leak is impossible by construction. The onError clear is registered as a SEPARATE router.onError call placed BEFORE the existing handler, so its early-returns for cancelled/aborted/duplicated/isCancelledFetch cannot skip the clear; the existing handler body is left untouched. AC5 restated from ''reference counted'' to ''a superseded navigation never strands the bar'', with an explicit cancelled-navigation leak test plus a redirect-chain edge case. Risk 3 upgraded to ''the highest-risk change in the ticket''. Ticket edge rule 3 is explicitly narrowed: navigations supersede rather than accumulate, so reference counting is wrong for them (it still applies to an in-flight query count if that is later folded in).'
status: addressed
---
