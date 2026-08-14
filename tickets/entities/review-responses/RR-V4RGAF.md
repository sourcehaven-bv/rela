---
id: RR-V4RGAF
type: review-response
title: Empty VTODO feed is byte-identical to an event feed in JSON
finding: |-
    VERIFIED: RenderJSON(Feed{Component: ComponentTodo}) and RenderJSON(Feed{}) both produce exactly {"events": []}. A consumer cannot distinguish "a to-do list with nothing due" from "an event calendar" — it will read `events`, find nothing, and show an empty calendar.

    Cause: json.go:25 puts `omitempty` on Todos, and an empty todo feed makes the slice with len 0, so the key vanishes entirely while `events` is always emitted.

    Fix: emit the component discriminator explicitly in the JSON so the consumer is not inferring the feed kind from key presence. (Dropping omitempty on Todos also works but leaves the kind implicit.)

    Related, lower stakes: CollectionTag(Feed{}) == CollectionTag(Feed{Component: ComponentTodo}) — both hash zero members, so both yield "4OYMIQUY7QOBJGX3". Harmless in practice because ctags are only ever compared WITHIN one collection and a collection never changes component kind. But mixing f.Component into the hash as a domain separator is free and removes the class of surprise. Same argument for ETag vs TodoETag, which cannot collide today only because they hash different renders.
severity: minor
resolution: |-
    Fixed in commit 5a0cac4e using the recommended approach: jsonFeed gained an always-emitted `component` field ("vevent"/"vtodo"), so a consumer never infers the feed kind from key presence. Chosen over simply dropping omitempty because that would have left the kind implicit.

    Pinned by TestRenderJSON_ComponentIsAlwaysExplicit, which also asserts directly that the two empty feeds are no longer byte-identical.

    The JSON path now also runs Todo.normalized(), so the iCalendar and JSON renderings can never disagree about the same Todo.

    NOT DONE — the ctag domain-separator suggestion. CollectionTag(Feed{}) still equals CollectionTag(Feed{Component: ComponentTodo}) for empty feeds. Deliberate: ctags are only ever compared WITHIN one collection, and a collection never changes component kind, so the collision is inert. Mixing Component into the hash would change every existing event-feed ctag for no behavioural gain, forcing a needless full re-sync on every subscribed client. Revisit only if a future surface compares tags across collections.
status: addressed
---
