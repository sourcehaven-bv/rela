---
id: RR-J04T1K
type: review-response
title: Action offer could run entity-less, shared in-flight state, redirect after unmount, missing correlation id
finding: Four defects in the new NextActionOffers action branch. (1) props.entityId is optional and runAction omits the request body entirely when it is absent, so an offer on an entity-less source ran the action against NO entity, silently and server-side. (2) `acting` was a per-instance ref, but NextActionCard and StatusBar both mount this component against the SAME singleton suggestion, so the banner and the status-bar popover could run the same mutation concurrently with no undo. (3) The redirect fired after two awaits with no liveness check, so a user who navigated away during a slow script was yanked to the script's destination. (4) The catch dropped the correlation id that the Sidebar precedent appends, leaving a toast with no handle on the server log — while the comment claimed 'same precedence as a list action'.
severity: significant
resolution: '(1) act() refuses without an entityId and the button is not rendered without one; pinned by ''renders no action button without an entity id''. (2) `acting` moved into useNextAction beside `busy`, module-level like the rest of the singleton, reset in __resetNextActionForTest. (3) An `alive` flag set false in onUnmounted now guards the router.push. (4) The error path appends `(ref: <id>)` from ApiError.correlationId, matching Sidebar.vue exactly, and the comment now names the sidebar rather than a list action.'
status: addressed
---

## Note on (2)

The per-instance ref was not merely redundant — `useNextAction` is explicitly a
module-level singleton because "two components render the same suggestion at
different prominences". Any state that guards a mutation on that suggestion has
to live at the same scope, or the guard is per-view rather than per-suggestion.

`busy` was already there and doing exactly this for feedback; `acting` should
have gone beside it from the start. The two stay separate because they disable
different controls and must not mask each other.
