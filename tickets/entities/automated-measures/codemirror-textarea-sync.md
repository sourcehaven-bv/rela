---
id: codemirror-textarea-sync
kind: test
location: internal/dataentry/static/app.js
status: active
title: CodeMirror textarea sync on changes
description: EasyMDE/CodeMirror editor syncs content to textarea on changes for HTMX form submissions
type: automated-measure
---

EasyMDE/CodeMirror editor automatically syncs content to the underlying textarea on every change using the batched `changes` event. This ensures HTMX form submissions always include the current editor content.
