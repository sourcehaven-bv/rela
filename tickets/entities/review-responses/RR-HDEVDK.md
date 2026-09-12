---
id: RR-HDEVDK
type: review-response
title: Inserting a reference and submitting immediately saved the body without it
finding: 'Milkdown''s markdown listener is debounced by 200ms. The entity-reference picker inserts by dispatching straight on the ProseMirror view, so a user who picked a reference and pressed Create within that window had the change land in the editor but never reach content.value — the entity saved with the pre-insertion body. Reproduced end-to-end: editor held ''## `FEAT-005` Summary...'' while the stored content was ''## Summary...''. Adding a 1s wait before submit made it persist, confirming the debounce as the cause.'
severity: critical
resolution: The editor exposes flush(), which runs the same decideEmit as the listener so it cannot bypass the write-back guard. DynamicForm calls it before handleSubmit reads content and before the autosave commitImmediately on navigate-away. Three unit tests pin it, including that a flush emits nothing when the only difference is round-trip churn.
status: addressed
---

## Finding

Found by running the e2e suite, which had never been executed. Two of its tests
failed on the same cause.

`@milkdown/plugin-listener` debounces `markdownUpdated` by 200ms. Typing is
unaffected in practice — nobody types and submits inside 200ms — but the toolbar
picker and the `@` menu both insert with a single `view.dispatch(...)`, and
pressing Create straight after is entirely normal.

Measured directly:

```text
EDITOR : "## `FEAT-005` Summary\n\nBrief description of the feature.\n"
STORED : "## Summary\n\nBrief description of the feature."
```

With a 1-second wait inserted before submit, the reference persisted. So the
insertion was correct and the save read a stale model.

## Why no unit test caught it

Milkdown's listener does not fire at all for a programmatic dispatch under
happy-dom, so every unit test sees the same "no emit" whether the wiring is
right or wrong. The debounce only exists in a real browser. This is the category
of defect the e2e suite is for, and it went unrun until now.

## Resolution

`flush()` on the editor, called by the form before it reads `content`:

- on submit, before `handleSubmit` builds its payload
- before `commitImmediately()` on navigate-away, which had the same exposure

It runs the same `decideEmit` as the listener, so it cannot be used to write
back churn or drift. Pinned by three tests, including one asserting a flush on a
churning body emits nothing.
