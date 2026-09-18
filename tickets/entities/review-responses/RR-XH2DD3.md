---
id: RR-XH2DD3
type: review-response
title: Extending button resolution to _views puts it on the world-capable surface for the first time; plan ignored worlds and faces
finding: '_sidepanel pins defaultViewWorld() with a comment that a surface must DECLARE its world rather than inherit one (sections.go:415-421), but _views is explicitly the one surface that opts into worlds (views_handler.go:523-527, viewWorldFromRequest). Extending resolveSectionButtonsWithTraverse to _views therefore puts it on a world-capable surface for the first time, and the resolver is world-blind: its signature takes entry *entity.Entity, not a viewWorld. The world decides which face a created entity lands in via worlds.<name>.create, and a faced type has no default row to fall back to, so omitting it is a refusal (BUG-HC6I2T, DynamicForm.vue:1400-1404). The plan mentioned neither worlds nor faces.'
severity: significant
resolution: 'Plan now documents the split: the page flow is correct for free because useWorld reads ?world= off the route and DynamicForm already carries it into the create payload and the redirect. The modal flow is the risk — useWorld reads the route, not the suppressed pre-fill query, so the host page''s world is the right one there, unlike the host''s link_* params. The distinction is subtle enough to break by accident, so the world is passed as an explicit prop with a test pinning that a modal create from a world-bound detail page lands at that world''s face. Also records that the resolver must run under the request''s world.'
status: addressed
---

## Finding

The plan proposed "generalizing" `resolveSectionButtonsWithTraverse` from
`_sidepanel` to `_views` without noticing that the two paths differ on worlds.

`_sidepanel` pins `defaultViewWorld()` with an explicit comment that a surface
must DECLARE its world rather than inherit one (`sections.go:415-421`). `_views`
is "the ONE surface that opts into worlds" (`views_handler.go:523-527`). So the
move puts button resolution on a world-capable surface for the first time, and
the resolver is world-blind — its signature takes `entry *entity.Entity`, not a
`viewWorld`.

This matters because the world decides which face a created entity lands in, via
`worlds.<name>.create`. A faced type has no default row to fall back to, so
omitting the world is a refusal rather than a silent default
(`DynamicForm.vue:1400-1404`, BUG-HC6I2T).

## Resolution

Plan now documents the split explicitly:

- **Page flow**: correct for free. `useWorld` reads `?world=` off the route and
`DynamicForm` already carries it into the create payload and the redirect
(`:1404`, `:1504-1514`).
- **Modal flow**: the risk. The embedded form suppresses the host's *query* for
pre-fill, but `useWorld` reads the **route** — and here the host's world is
exactly the right one, unlike the host's `link_*` params. That distinction is
subtle enough to be broken by accident, so the world is passed as an explicit
prop and a test pins that a modal create from a world-bound detail page lands at
that world's face.

Also recorded: the resolver must run under the request's world, or its derived
targets can describe rows the user is not looking at.
