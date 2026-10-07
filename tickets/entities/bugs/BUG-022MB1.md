---
id: BUG-022MB1
type: bug
title: Detail panel goes stale after a relation edit elsewhere
description: |-
    With relation-backed status (TKT-ZNFGNJ), moving a task to another column on a kanban re-points its status relation. The task's open detail panel kept showing the old status until a manual reload. Two causes: pumpStoreEvents in internal/dataentry/watcher.go broadcast entity:changed only for entity events, so a relation write sent no change for either end type; and EntityDetail.vue did not listen for entity:changed at all, so it reloaded only after its own edits.

    Fix: relation events now broadcast entity:changed for the relation's from and to types (relationEndTypes, deduplicated). EntityDetail reloads its view on entity:changed for its type, unless content autosave has pending writes, and after an inline relation edit (onRelationChanged).
priority: medium
effort: s
why1: A relation write produced no entity:changed event, and the detail panel did not subscribe to entity:changed, so nothing told the panel its data had changed.
why2: The SSE pump mapped store events to UI events by entity type only. Relation events have no entity type of their own and were dropped.
why3: Until relation-backed status, a relation change elsewhere rarely changed what a detail panel showed, so no view depended on relation events and the gap stayed invisible.
why4: The detail panel assumed it was the only writer of its entity and reloaded only on its own saves.
why5: There is no rule that every store event maps to at least one UI change event, and no test that a relation write reaches the views that show either end.
prevention: Relation events broadcast for both end types, with a test in watcher. Any new view that renders relation data subscribes to entity:changed for its type, as list, kanban and detail views now do.
status: backlog
---

Fixed on branch `demo/relation-backed-status` together with TKT-CADCFX.
