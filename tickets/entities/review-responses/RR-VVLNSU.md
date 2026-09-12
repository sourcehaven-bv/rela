---
id: RR-VVLNSU
type: review-response
title: Incoming RelationPicker keeps record 1's selection through the reset and re-writes it as new links
finding: An incoming-direction RelationPicker stores its selection in its own incomingValue ref (RelationPicker.vue:88) and effectiveValue (:134) ignores props.value, so clearing the parent's relations does not clear it. Record 1's peers then re-emit as additions against an empty incomingOriginal and are written to record 2 as duplicate links.
severity: critical
status: addressed
resolution: >-
  Plan step 1a added: the saveGeneration bump is now required (not belt-and-braces) because it remounts the incoming picker. New AC-4d asserts an incoming picker's selection does not reach record 2's payload.
---

## Finding

For `direction: incoming`, `RelationPicker` holds the selection in its OWN
`incomingValue` ref (`RelationPicker.vue:88`), and `effectiveValue` (`:134`)
reads it while ignoring `props.value` entirely. Clearing the parent's
`relations` in the reset therefore does nothing.

Worse, `emitIncomingDiff()` diffs against `incomingOriginal`, which the
create-mode branch (`:184-190`) seeded to `[]` — so record 1's still-selected
peers re-emit as pure additions and are written to record 2 as duplicate links.

Silent wrong-data write, on a widget no acceptance criterion covered.

## Resolution

The `saveGeneration` bump is the fix (it is in the picker's `:key` at
`FormFieldList.vue:93`), which makes finding RR "saveGeneration is dead code"
load-bearing rather than belt-and-braces. Add an explicit acceptance criterion
for an incoming picker across a reset.

Verified against source before accepting.
