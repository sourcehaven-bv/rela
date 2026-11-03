---
id: RR-CSH2KD
type: review-response
title: Widening prop values to string[] is not caught by vue-tsc at the type-guard sites
finding: 'The risk table (plan lines 705-708) relies on vue-tsc to list every site. User-defined type guards that test only `typeof v === ''object''` compile unchanged and will classify an array as a ref: RlChannelMessage.vue:152-154 `isRef` (renders a list as a ref chip with undefined id), channelView.ts:107-109 `isReadableRef` (feeds markdown ref links), RlChannel.vue:183 and channelView.ts:524 (excerpt parts print `v.title ?? v.id` = undefined). Other sites lose data: channelValues.ts:190-194 `carryProp` returns `value.id` for an array, so editing a message whose list field is not rendered silently erases the list (toEditProps carries hidden props through carryProp); useSessionDecisions.ts:329-335 `presetProps` drops arrays the same way. Fix: name these sites in step 9/10, add an `Array.isArray` branch BEFORE every object check, and add tests: list prop carried through an edit when hidden, list renders as items not a chip.'
severity: significant
resolution: Steps 7 to 9 name isRef, isReadableRef, both excerpt loops, carryProp, isRestrictedRef and presetProps, each with an Array.isArray branch before the object check. AC 12 adds tests for a hidden list carried through an edit and for a list rendering as items, not a ref chip.
status: addressed
---
