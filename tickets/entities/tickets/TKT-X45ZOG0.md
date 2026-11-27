---
id: TKT-X45ZOG0
type: ticket
title: Options to hide type and ID on cards and mention lists
kind: enhancement
priority: medium
effort: s
started: "2026-11-26"
completed: "2026-11-26"
status: done
---

## Description

Relation cards and the "Mentioned by" panel always show the entity type and
internal ID. Add `hide_type` and `hide_id` on view sections and
`mentioned_by_hide_id` on entity views. Defaults keep current output.

## Follow-up

Issue #2626: the world-absent view response set `hideMentionedByPanel` but not `hideMentionedByID`, so `mentioned_by_hide_id` had no effect there. `writeWorldAbsentView` now sets both, and `TestFaceOrder_WorldAbsentPageUsesDeclarationOrder` checks it.
