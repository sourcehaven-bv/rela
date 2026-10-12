---
id: BUGA-RFV4PD
type: bug-analysis-checklist
title: 'Bug Analysis: Detail panel goes stale after a relation edit elsewhere'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally: on the demo server, open a task's detail panel, then drag the card to another column on the board. The panel kept the old status.
- [x] Minimal reproduction steps documented: see above; any relation write made elsewhere shows it.
- [x] Environment/conditions noted: rela-server with examples/relation-status-demo, any store; happens only for relation writes, not property writes.

## Root Cause

- [x] Immediate cause identified (why1): no entity:changed event for a relation write, and EntityDetail did not listen for entity:changed.
- [x] Contributing factors found (why2-3): pumpStoreEvents in internal/dataentry/watcher.go mapped store events by entity type only, so relation events were dropped; no view depended on relation events before relation-backed status.
- [x] Systemic cause explored (why4-5): the detail panel assumed it was the only writer; no rule or test that every store event reaches the views that show either end.

## Fix Planning

- [x] Fix approach determined: broadcast entity:changed for the relation's from and to types (relationEndTypes, deduplicated); EntityDetail reloads on entity:changed for its type unless autosave has pending writes, debounced.
- [x] Regression test planned: TestRelationEndTypes in watcher_relation_test.go and the listener tests in EntityDetail.events.test.ts.
- [x] Related areas checked for similar issues: list and kanban views already subscribe to entity:changed; section forms keep pending edits across a reload.
