---
id: RR-CZOHDW
type: review-response
title: Cmd+Enter is dead in the inline-create modal
finding: DynamicForm.vue skips registering its document keydown listener when embedded, with a comment saying "the host modal owns Cmd+Enter" — but InlineCreateFormModal handled only Escape. No Enter/metaKey handler existed anywhere in it. The form still rendered the <kbd>⌘↵</kbd> hint on its Create button, advertising a shortcut that did nothing, and the AC7 e2e test for it was never written.
severity: critical
resolution: 'DynamicForm now defineExposes { isDirty, isSaving, submit }; the modal''s keydown handler calls submit() on Cmd/Ctrl+Enter. Pinned by three tests in InlineCreateFormModal.test.ts (Cmd+Enter, Ctrl+Enter, and a bare Enter that must NOT submit). Mutation-checked: removing the handler fails them.'
status: addressed
---
