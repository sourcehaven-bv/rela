---
id: RR-HS1TRV
type: review-response
title: Resolving only at mount leaves a post-mount reload's new id typeless
finding: resolveOutOfPageLinks ran once in onMounted. FormFieldList keys the picker on DynamicForm's saveGeneration, which is never incremented, so a post-mount entity reload reassigns relations.value WITHOUT remounting the picker. An id arriving that way would be in neither candidates nor resolvedLinks, so buildOutgoingTypes yields no type and the save aborts — reproducing the exact bug on a path the fix claimed to cover.
severity: critical
resolution: 'Verified the claim before acting: saveGeneration is declared at DynamicForm.vue:250 and never assigned, and two live post-mount reload paths exist (loadEntity(true) after a committed state-machine transition, and onAttachmentChanged). Added a watch on the linked-id set that re-runs the resolve and re-emits update:types — the re-emit is essential, since resolving a type the parent never receives fixes nothing. Pinned by ''resolves an out-of-page id that arrives after mount'', written failing first.'
status: addressed
---

The reviewer's reasoning was correct and the failure is reachable in the
product, not just in theory.

Worth recording why the first fix missed it: the component already had this
assumption in its pre-existing `onMounted` emit, and I treated matching the
existing lifecycle contract as sufficient. It was not. The pre-existing emit
sharing the assumption made the bug *old*, not absent — and this ticket is about
precisely that failure class, so inheriting it was the wrong call.
