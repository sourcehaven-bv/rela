---
id: RR-5QXE
type: review-response
title: Picker can be opened multiple times via rapid keyboard — no debounce on toolbar action
finding: |-
    The toolbar action handler in `MarkdownEditor.vue:53-55`:
    ```ts
    action: () => {
      pickerOpen.value = true
    },
    ```

    No guard against re-entry. If the user keyboard-shortcuts through the toolbar fast enough, or if a CodeMirror keymap fires the action twice (it has happened in EasyMDE in the past via keybind chains), `pickerOpen` is set to true twice. The picker's `watch(props.open)` with `flush: 'sync'` fires on the FIRST transition false→true, then is a no-op on the second `true→true`. So the visible behavior is fine: one picker.

    BUT, the picker's `previouslyFocused` capture (`document.activeElement`) runs on the false→true transition. If the picker is closed and immediately re-opened in the same tick (e.g. select → close → user keyboard-triggers again before focus returns), the `previouslyFocused` capture grabs whatever the activeElement is mid-transition — possibly the body, possibly the toolbar button, possibly the editor itself — and restores to it on close. The result is a focus-return that surprises the user.

    Not a critical bug, but worth a defensive check: if `pickerOpen.value === true` already, skip the action. And consider documenting that the picker's reopen-from-itself semantics are undefined.

    The e2e suite has no rapid-fire reopen test; CommandPaletteModal had a similar bug fixed only after it was reported by a user.
severity: nit
reason: 'Today benign: setting pickerOpen.value = true while already true is a no-op for the modal stack (idempotent watch) and the modal itself (v-if doesn''t re-mount on same value). Not worth the guard.'
status: deferred
---
