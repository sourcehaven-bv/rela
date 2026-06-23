---
id: RR-FA1C
type: review-response
title: Content-only instance with empty-ref formData and computed contentRef
finding: |
  EntityDetail will pass formData: ref({}) and contentRef: computed(() => entry.value?.content ?? ''). mergeServerResponse writes lastSeenContent = entity.content (a local -- fine) and calls opts.applyServerContent(entity.content) (the callback -- fine). It does NOT mutate contentRef.value directly. However: contentRef appears unused in the composable's body. That's a dead-code smell that means passing a computed is safe today but doesn't lock in the contract.
severity: significant
status: addressed
resolution: |
  Verified by reading useAutoSave.ts in full: opts.contentRef is declared in AutoSaveOptions but never read inside the composable. EntityDetail can pass a computed ref safely. Add a jsdoc comment to AutoSaveOptions.contentRef clarifying it's read-only-shape (composable never mutates it). Removing it entirely is out of scope for IHC7A (touches DynamicForm) -- flagged as a follow-up cleanup. AC 4 amended: "EntityDetail passes contentRef: computed(() => entry.value?.content ?? '') -- safe because composable never mutates contentRef."
---
