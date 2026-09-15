---
id: milkdown-raw-html-no-dom-sink
type: automated-measure
title: 'Test: Milkdown renders raw HTML as text, never through a DOM sink'
description: 'Control for GitHub #1597. CommonMark''s raw-HTML passthrough keeps literal HTML from an entity body as an `html` node, so the bytes reach the editor; it is safe only because that node''s toDOM returns the value as a text CHILD (ProseMirror createTextNode), not via innerHTML. We do not own that behaviour, and Milkdown has shipped this bug class twice in adjacent nodes (CVE-2026-57530 link href, CVE-2026-57531 emoji innerHTML sink, fixed 7.21.3; pinned at 7.22.1). The suite mounts the real editor and asserts observable consequences - no element created for <img onerror>/<script>/container tags, no navigable javascript: href, and lossless round-trip back to stored markdown - so it survives a reimplementation of the node and fails on a dependency regression. Verified non-vacuous: patching the preset html node to assign span.innerHTML fails 3 of 5 tests; making sanitizeLinkHref an identity function fails the fourth.'
kind: test
location: frontend/src/components/forms/milkdown/rawHtmlPassthrough.test.ts
status: active
---
