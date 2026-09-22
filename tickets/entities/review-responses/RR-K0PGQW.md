---
id: RR-K0PGQW
type: review-response
title: ToggleLink destroys an overlapping link and discards the entered URL
finding: 'ToggleLink is toggleMark, whose removeWhenPresent defaults true and computes add = !ranges.some(rangeHasMark) — .some, not .every. So a selection partially overlapping an existing link takes the REMOVE branch: the existing link is destroyed and the URL the user just typed is silently discarded, discovered only at save. Verified at prosemirror-commands/dist/index.js:700-701.'
severity: critical
resolution: The dialog tests whether the selection intersects a link mark before opening. If it does, it prefills with that href and commits via UpdateLink (retarget the whole link); only a link-free selection takes the ToggleLink insert path. Pinned by AC 11. This also supplies the keyboard route to retarget.
status: addressed
---

**Finding (design review of PLAN-RY25IP).** The plan listed "selection spanning
a link and plain text → defined behaviour asserted" as an edge case but never
defined the behaviour. It is worse than an unhandled case: it is silent data
loss.

`ToggleLink` is `toggleMark(linkSchema.type(ctx), payload)`
(`preset-commonmark/lib/index.js:376`). ProseMirror's `toggleMark` defaults
`removeWhenPresent: true` and decides:

```js
add = !ranges.some(r => state.doc.rangeHasMark(r.$from.pos, r.$to.pos, markType))
```

`.some`, not `.every`. If ANY part of the selection already carries a link mark,
`add` is false and the command removes. A user who selects across `see
[docs](url) here` and presses the link button loses the existing link, and the
URL they entered in the dialog is discarded. Nothing surfaces until save.

**Resolution.** Before opening the dialog, test whether the selection intersects
a `link` mark. If so, prefill with that link's href and commit through
`UpdateLink`, retargeting the whole link — which is what `UpdateLink` does
regardless, since it rewrites over the matched node's full `nodeSize`, so a
partial selection cannot produce a partial retarget. Only a selection carrying
no link takes the `ToggleLink` insert path.

Pinned by AC 11. As a side benefit this makes the toolbar button
context-sensitive (caret in a link → edit it), which is the keyboard-reachable
route to retarget now that `Mod-k` is out of scope.
