---
id: RR-8AOV
type: review-response
title: previouslyFocused.focus() does not check focusability — silently fails for non-focusable elements
finding: |-
    `EntityPickerModal.vue:121-123`:
    ```ts
    if (prev?.isConnected) {
      prev.focus()
    }
    ```

    `isConnected` checks the element is in the DOM. It does NOT check the element can receive focus. If the captured `previouslyFocused` is, say, a `<div>` with no `tabindex`, or an element whose `tabindex` was removed between capture and restore, `.focus()` is a no-op (modern browsers silently fail; some legacy browsers throw).

    More importantly: when the picker opens via the toolbar button, `document.activeElement` at that moment IS the toolbar button — EasyMDE's toolbar buttons have implicit focus on click. So `previouslyFocused` is the button. On close, the picker's watcher focuses the button. Then `MarkdownEditor.onPickerClose` schedules `editor?.codemirror.focus()` in `nextTick`. Net result: focus jumps button → editor in one frame. Usually invisible, but if EasyMDE's button has focus-visible styling it briefly flickers.

    When the picker opens via a hypothetical future keyboard shortcut (Cmd+@?), `previouslyFocused` could be the CodeMirror textarea itself. On close, the picker focuses the textarea. Then `nextTick` focuses it again. No-op the second time — fine.

    When `previouslyFocused` is `<body>` (modal opened from a synthetic event with no prior focus), `body.focus()` is a no-op — fine.

    Low severity. The robustness fix is to check `typeof prev.focus === 'function' && prev !== document.body` before calling, but the current code is correct for the common cases.
severity: nit
reason: MarkdownEditor explicitly re-focuses the CodeMirror textarea in nextTick after the modal closes (RR-SKX3), overriding the modal's previouslyFocused restore. The picker's restore-path is therefore decorative for this consumer; the CommandPaletteModal pattern is preserved unchanged so future consumers keep its semantics.
status: deferred
---
