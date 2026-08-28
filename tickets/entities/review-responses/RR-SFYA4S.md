---
id: RR-SFYA4S
type: review-response
title: Calendar query key broke SSE invalidation and optimistic updates
finding: CalendarView keyed its query on a calendar-namespaced key that does not prefix-match the entity-type key useEvents invalidates on SSE events. The grid therefore never refreshed on remote changes. Separately beginOptimistic writes to entityKeys.list(type) — a different cache entry — so the optimistic drag update never rendered and only the post-mutation refetch moved the chip.
severity: significant
resolution: Publish each source fetch under entityKeys.listParams(type params) which descends from entityKeys.list(type) so both the SSE invalidation and the optimistic write reach it by prefix. Pinned by a new cache-key test verified to fail against the old key.
status: addressed
---

## Finding

`CalendarView.vue` keyed its data query on
`['entities','calendar',<id>,<view>,<date>,<tz>]`. That key reads naturally but
silently breaks two mechanisms the component depends on:

- **SSE invalidation misses the grid.** `useEvents` invalidates
`entityKeys.type(<type>)` = `['entities',<type>]` on every entity event. That
prefix does not match a key whose second element is the literal `'calendar'`, so
an entity changed by another user (or another tab) never refreshed the calendar.
- **The optimistic update targeted a different cache entry.**
`beginOptimistic` writes through `entityKeys.list(<type>)`. The grid read from
the calendar-namespaced entry instead, so a dragged event would not move until
the post-mutation refetch landed — the optimistic path was dead code that still
paid for a rollback.

Neither failure is visible in an ordinary render test: the grid shows the right
events either way. This is precisely the hazard the design review raised as S1 —
the caching half was addressed and the invalidation half was reintroduced.

## Resolution

Fetch each source and publish it under `entityKeys.listParams(type, params)`,
which descends from `entityKeys.list(type)`. The params segment still gives each
window its own cache entry, while both the SSE invalidation and the optimistic
write now reach it by prefix.

Added `CalendarView.test.ts` → "CalendarView cache keys", which asserts every
entry the grid populates sits beneath `['entities',<type>,'list']`. Verified the
test FAILS against the old key (`expected 0 to be greater than 0`) — the
contract had no test before, which is why the bug survived.
