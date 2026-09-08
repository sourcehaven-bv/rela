---
id: BUG-CV8L3B
type: bug
title: A denied ?world= short-circuits past worldCapablePath, serving default-world content on refused routes
description: 'In attachWorld the errWorldDenied branch calls next.ServeHTTP immediately, before the `if !handle.isDefault() { if !worldCapablePath(...) }` refusal that runs on the resolved branch. A principal WITHOUT the world grant therefore reaches non-world-capable routes that a principal WITH the grant is refused on (422 world_unsupported). Those routes are world-blind, so the denied principal gets the DEFAULT world''s answer. Two effects: on /api/v1/_analyze that answer is 200 carrying default-world entity ids and titles (content disclosure), and across every refused route the denied and permitted status codes differ (422 vs 200/400/404), so the response is a grant oracle.'
priority: high
effort: s
why1: The errWorldDenied case in attachWorld returns via next.ServeHTTP before the route-capability check, so a denied world never sees worldCapablePath.
why2: The denied handle was designed to make the request continue so the ordinary handler renders its own empty result (closing the existence oracle), and continuing was implemented as an early short-circuit rather than as a value flowing into the same post-resolution checks.
why3: Route capability is a property of the ROUTE, but it was placed inside `if !handle.isDefault()`, a branch keyed on the resolution OUTCOME — so a question that does not depend on the principal was made to depend on it.
why4: Only four seams consult blocksAllReads (scopedSortedEntities, freeTextIDsForType, visibleReader.getVisible, worldneighbors). Routes outside the world-capable allowlist were never taught the handle, correctly, because the allowlist was supposed to keep them unreachable under any non-default world.
why5: The allowlist's deny-by-default guarantee had two entry points but only one enforcement point. Nothing structurally tied 'this route may serve a non-default world' to 'a non-default world was requested' — it was tied instead to 'a non-default world resolved successfully', and the denied handle is a non-default world that did not.
prevention: 'Decide route capability on the REQUESTED world before the grant outcome can branch the response, and pin it with a table test that asserts permitted and denied produce an identical response on a non-world-capable route. The identical-response assertion is the durable guard: it fails for any future divergence between the two, not just this one.'
status: done
---

## Reproduction

Against `develop` @ `169de983`, in `internal/dataentry`:

```go
app := newTestAppV1(t)
seedEntity(app, &entity.Entity{
    ID: "TKT-900", Type: "ticket",
    Properties: map[string]any{"title": "secretdraft"},
})
role := acl.RoleDef{Read: []string{"ticket"}}   // no Worlds grant
app.acl = mustNewACL(t, &acl.Policy{
    Roles:       map[string]acl.RoleDef{"viewer": role},
    Assignments: map[string]string{"alice": "viewer"},
}, app.store)
app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}, resolveDefault: true})
```

With `?world=published`, permitted vs denied:

| route | permitted | denied |
|---|---|---|
| `_analyze` | 422 `world_unsupported` | **200** `{"issues":[{"entityId":"TKT-900",…,"title":"secretdraft"}]}` |
| `tickets/TKT-900/relations` | 422 | **200** `{}` |
| `_documents/report` | 422 | **404** `document_not_found` |
| `_position` | 422 | **400** `bad_request` |
| `tickets/TKT-900/_export` | 422 | **400** `missing_transform` |

## Two defects, one cause

1. **Content disclosure.** `_analyze` is world-blind. A caller who asks for `published` and is denied it receives the *default* world's entities — the draft content the world exists to withhold. This is a value disclosure, not the accepted one-bit membership channel.

2. **Grant oracle, inverted.** On *every* refused route the denied and permitted responses differ, so the status code alone tells an unprivileged caller whether they hold `world:published`. `resolveWorld`'s doc comment is explicit that a denial must be indistinguishable from an empty world; here it is distinguishable from a *refusal*, in the direction that hands over more. The disclosure is confined to `_analyze`; **the oracle is on all five.**

The comment above the allowlist states the intended posture: "a leak requires
someone to WIDEN this predicate, a visible and reviewable act, rather than to
forget a call site." The denied branch is a second way in that does not consult
the predicate at all.

## Fix

Extract `refuseWorldIncapablePath`, keyed on the REQUESTED world name, and call
it after the duplicate/unknown 400 arms and before every arm that consults the
grant. Both a permitted and a denied `published` then get the same 422, and the
denied handle continues to flow only into routes that are world-scoped and
tested.

Keyed on the name rather than a resolved handle because a denied handle carries
the zero scope, and a zero scope IS the default world — `handle.isDefault()`
cannot distinguish "no world asked for" from "asked for and refused".

Ordering: capability is decided on the requested world, so the refusal cannot
itself become an oracle — it is reached identically whether or not the grant
would have passed. It sits *below* the two 400s so an operator keeps the precise
`no world named "pubished" is declared` diagnostic on every route.
