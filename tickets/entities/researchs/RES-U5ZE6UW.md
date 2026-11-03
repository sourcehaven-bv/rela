---
id: RES-U5ZE6UW
type: research
title: Inbox rows that act on waiting sessions
summary: Store the session cause and batch-read waiting messages so inbox entries carry the waiting message and decision inputs; the SPA reuses the entity page decision builders; form decisions open the channel in a drawer.
status: done
---

## Problem

How does an inbox row get the waiting message, its decision buttons and its
choices, so a person can answer a waiting session without opening the entity
page? The entity page builds them in the SPA from the channel head, the caller's
`_actions` at the head's generation, and the held messages (the waiting message
is found through the session notice's `cause`). The inbox has none of the
messages and only part of the affordances, and the server does not store which
message moved the session.

## Context

- Collection reads must be batched per page; no per-row lookups
(`.claude/rules/collection-reads.md`).
- The inbox is per principal; affordances must match the entity page.
- Decision and choice logic: `channelView.ts` (`decisionViews`,
`choiceViews`, `decisionCause`) and `useSessionDecisions.ts`; server
affordances: `call.affordancesWith`, `call.sessionAffordances`.
- The inbox already calls `sessionAffordances(postable())` per entry.
- The session notice is appended at cause seq + 1, but neither seq is stored
on the session.

## Options

A. Server enriches the entry, SPA reuses the builders. Store the cause seq on
the session (`Session.Cause`, new column on pg and sqlite), read the waiting
messages of a page in one batch (`Store.GetMessages`), compute decision inputs
with the head's function. The SPA runs the same builders and engine over the
entry. Form decisions open the real channel in a drawer. Effort l.

B. SPA loads each channel (head + newest page per row). Full reuse, but two
requests per row per replay. Violates the collection-read rule. Rejected.

C. Lazy expand: a row mounts the channel when expanded. Cheap, full reuse, but
the question is not visible on the row. Used only for form decisions.

D. Find the cause without storing it. Needs a tail read per channel. Rejected
like B.

E. Inline composer on the row. Needs a second copy of the composer glue in
OpenChannel. Rejected; the drawer reuses it.

## Recommendation

A, with C for form decisions. Accepted tradeoffs: a store interface change on
three backends; sessions moved before the change show no message until their
next move; summary-only live rounds pay one batch message read.
