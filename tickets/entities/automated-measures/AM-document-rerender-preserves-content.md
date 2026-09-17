---
id: AM-document-rerender-preserves-content
type: automated-measure
title: A document re-render never unmounts the rendered body, and an unchanged render performs no DOM write
description: Component tests asserting a document re-render never falls through to the empty state, and that a byte-identical render preserves .document-body DOM node identity.
kind: test
location: frontend/src/views/DocumentView.rerender.test.ts
status: active
---

## Measure

Component tests on `DocumentView.vue` pinning both halves of the anti-flash
contract that `frontend/src/views/DocumentView.vue:37-39` already states in
prose but did not enforce:

1. **Re-render keeps the previous document mounted.** Drive an `entity:changed`
event with a slow-resolving render and assert that at no point during the
in-flight fetch does the empty state (`No document content available`) appear,
and that the `.document-body` element is never unmounted. Asserting on the
rendered branch rather than on `docContent` catches the fall-through to the
`v-else` branch, which is the actual user-visible defect.

2. **An unchanged render is a no-op.** Re-render returning byte-identical HTML
must not reassign `docContent`, verified by holding a reference to the
`.document-body` DOM node across the event and asserting node identity is
preserved. Node identity is the right assertion because it is what determines
whether the browser resets scroll and whether the `sanitizedContent` watcher
re-runs Mermaid/PlantUML.

## Why this shape

The original bug was invisible to any assertion on `docContent` alone - the
value is correct before and after; the defect lives in the transient state
*during* the fetch. A test that awaits the promise and checks the end state
passes against the buggy code. The measure therefore has to observe the
intermediate render.
