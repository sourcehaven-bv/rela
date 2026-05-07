---
id: RR-UPK8J
type: review-response
title: 'RelationCards ''just call savePendingRelationCards eagerly'' is wrong: it remounts via saveGeneration and re-emits cumulative diff'
finding: |-
    Plan section 3 claims: "The existing savePendingRelationCards logic at DynamicForm.vue:411-432 already does the right calls — we just call it eagerly per card change." Two problems:

    (a) savePendingRelationCards ends with saveGeneration.value++ (line 432). The form template uses saveGeneration in <RelationCards :key=...> (lines 575, 606), forcing a remount. With auto-save firing per change, RelationCards would remount mid-interaction — destroying SlimSelect state, popovers, scroll, focus, and pending property edits. RelationCards.vue:161 has an explicit comment about preserving component instances for exactly this reason.

    (b) RelationCards.emitUpdate emits the *cumulative* RelationCardState since last load (added/removed/updated as accumulators). Eager save would re-emit the same entries on every change, causing duplicate createRelation/updateRelationProperties calls.

    Proper fix: refactor RelationCards to support an autoSave prop where it persists per-row immediately and clears its own diff atomically (no diff accumulator when autoSave is on), or expose a flush+reset method that bypasses the saveGeneration remount. 0.5d estimate is wrong; this is real work.
severity: critical
resolution: 'Split RelationCards autosave into TKT-B9SXH. This ticket explicitly does NOT change the cards-changed flow; relation-cards fields in auto_save: true forms continue with deferred-save behavior until TKT-B9SXH lands. Plan notes a console-warning guardrail during dev when both flags coexist.'
status: addressed
---
