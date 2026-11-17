---
id: TKT-M07C97Y
type: ticket
title: One step at a time over a set of entities
kind: enhancement
priority: medium
effort: m
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

Follow-up to TKT-35BY3PE (workshop step mode). A step on a `mode: steps` page
that walks the rows of a list one by one, for example "for each processing
activity, answer these questions" in a DPIA workshop.

- The step shows one entity at a time, in the entity's own detail or form
view, with its position ("3 of 8") and previous/next entity controls.
- In present mode, PageDown and PageUp move through the entities first, then
on to the next or previous step.
- The URL is the source of truth: the current entity is in the URL, so a
reload, a bookmark and the browser's back button land on the same entity.
- Rows come from the existing ACL-gated list API, so a hidden entity is never
shown or counted beyond what the list already shows.

The config shape is chosen in planning and recorded in the planning checklist.
