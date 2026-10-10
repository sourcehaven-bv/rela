---
id: TKT-WA25G2
type: ticket
title: Comment and relation counts on kanban cards
kind: enhancement
priority: medium
effort: m
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: Let kanban card fields show the number of comments and the number of related entities (such as subtasks), batched per page and ACL-gated.
---

## Description

A kanban card cannot show how many comments or subtasks an item has. Card fields
show property values or relation targets only (Atlas TASK-T7342; the board
mockup shows a comment and a subtask count on every card).

The library card already has the visual (`RlMetaItem` with an icon and count,
used by `RlTaskCard` for `commentCount` / `subtaskCount`).

## Acceptance criteria

- A card field `relation: <name>` with `display: count` shows the number of
related entities the reader may see, as an icon and number.
- A card field `comments: true` shows the number of comments on the card's
entity, only when the reader may read comments on it.
- Counts are computed per page in one batch, not per card.
- A hidden related entity is not counted; a reader without `comment:read`
sees no comment count.
- Invalid combinations are config errors at load.
- docs and e2e cover both.
