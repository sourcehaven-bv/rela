---
id: RR-UTURB1
type: review-response
title: Nesting cap is accidental, not enforced — modal-in-modal happens for free
finding: The plan caps nesting via "nested forms render only RelationPicker because cards need entityId" — but that caps edge-meta, not nesting. useInlineCreate is driven purely by targetTypes and the schema store; nothing in it knows its depth, so a nested form's RelationPicker offers inline create identically, giving modal-in-modal. modalStack.ts is a Set (isAnyModalOpen returns size > 0), not a stack — it cannot answer "am I topmost", so Escape has no defined recipient and there is no z-index ordering. Listing nesting as "out of scope" enforces nothing.
severity: significant
resolution: 'Cap made structural: InlineCreateFormModal provides inlineCreateDepth (parent + 1); useInlineCreate injects it (default 0) and returns no eligible types at depth >= 1, making modal-in-modal unreachable. Added as AC10 with a Vitest case.'
status: addressed
---

## Resolution

The cap is made structural. `InlineCreateFormModal` provides an
`inlineCreateDepth` (parent depth + 1); `useInlineCreate` injects it (default 0)
and returns **no eligible types at depth >= 1**. A nested form therefore offers
link-existing only.

This is ~5 lines, removes the undefined Escape/z-index behaviour entirely by
making modal-in-modal unreachable, and gives a directly testable criterion.
Added as AC10 with a Vitest case asserting the composable returns an empty list
when depth is injected as 1.
