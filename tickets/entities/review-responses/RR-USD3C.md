---
id: RR-USD3C
type: review-response
title: MarkdownEditor watcher destroys caret on external value sync — the per-field merge will clobber typing
finding: |-
    MarkdownEditor.vue:57-64 has a watcher on props.modelValue that calls editor.value(newValue) whenever the prop changes. EasyMDE's setValue is destructive to caret position. With auto-save merging PATCH responses back into content (per finding on response merge), the watcher fires editor.value(...) mid-typing → caret jumps to start, unflushed local typing erased.

    Fix: MarkdownEditor needs to skip external sync when the editor is focused, or accept a skipExternalSync prop the form sets while content is in its dirty window. Test: type a paragraph, fire SSE refresh during typing, assert caret position and content both preserved.
severity: significant
resolution: 'MarkdownEditor.vue modelValue watcher made focus-aware: skips editor.value(newValue) when the editor is focused (preserves caret + unflushed typing). External sync resumes once the editor blurs. AC #14 has a Playwright test that types into the editor, fires response merge, and asserts caret/content unchanged.'
status: addressed
---
