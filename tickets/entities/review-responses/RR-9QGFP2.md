---
id: RR-9QGFP2
type: review-response
title: Inline-create depth cap fails open; DuplicateModal must call provideInlineCreateDepth()
finding: 'Depth and the `embedded` prop are decoupled. `embedded` is a DynamicForm prop; depth comes only from provideInlineCreateDepth(), called in exactly one place (InlineCreateFormModal.vue:71). A DuplicateModal mounting <DynamicForm embedded> without calling it leaves depth at 0, so useInlineCreate (useInlineCreate.ts:63) returns a populated list and every RelationPicker/RelationCards inside the duplicate form offers ''+ New X''. Clicking opens InlineCreateFormModal on top of the duplicate modal — the modal-in-modal the cap exists to make unreachable. useInlineCreate.ts:17-22: the cap is STRUCTURAL because modalStack is a Set, not a stack, so Escape has no defined recipient and both overlays sit at the same z-index (InlineCreateFormModal z-index: 900).'
severity: critical
resolution: Ticket now states DuplicateModal calls provideInlineCreateDepth(), with AC10c, and records the consequence that the duplicate form offers link-existing only as a stated decision.
status: addressed
---

## Resolution required

State explicitly that `DuplicateModal` calls `provideInlineCreateDepth()`, and
add an AC — none of the original 20 covered it.

Also record the second-order consequence as a stated decision rather than
something discovered later: doing this correctly means the duplicate form's
relation pickers offer link-existing only, not inline-create. That is the right
trade, but it is a real UX consequence.
