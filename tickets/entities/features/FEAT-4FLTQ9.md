---
id: FEAT-4FLTQ9
type: feature
title: 'Piles: personal working sets of entities'
summary: Collect entities from any list or search into a named personal pile; step through it, run configured actions on it and export it.
description: Let a user collect entities from any list or search into a named personal pile, then step through it, run operator-configured actions on it and export it.
priority: medium
status: proposed
---

## Goal

Let a user collect entities from any list or search result into a named,
personal set (a "pile"), and then work through that set: step through it on the
entity page, run operator-configured actions on it, and export it.

The idea comes from Tornado Notes (DOS, 1986), where a search produced a pile
you could flip through. Atlas tracks the request as TASK-KXEES.

## Scope

- Personal piles: one user's working sets, not shared.
- Adding from list and search selections; a sidebar entry per pile.
- A pile as a navigation scope next to list and search.
- Operator-configured actions and exports on a pile.

Sharing is out of scope by design: a collection a team should see is graph data
(an entity with relations), not a pile.
