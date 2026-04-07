---
id: codemirror-textarea-sync
kind: test
location: frontend/src/components/forms/MarkdownEditor.vue
status: active
title: CodeMirror editor syncs to v-model on change
description: EasyMDE/CodeMirror editor emits update:modelValue on every change so Vue form state always reflects the current editor content
type: automated-measure
---

EasyMDE replaces a textarea with a CodeMirror editor. The Vue `MarkdownEditor.vue`
component registers a `codemirror.on('change', ...)` listener that calls
`emit('update:modelValue', ...)` with the current editor value, keeping the
parent component's reactive state in sync with what the user sees in the editor.
This ensures form submissions always include the current editor content.
