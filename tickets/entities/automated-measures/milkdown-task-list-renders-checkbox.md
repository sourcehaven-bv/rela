---
id: milkdown-task-list-renders-checkbox
type: automated-measure
title: 'Test: the Milkdown editor renders GFM task lists as real checkboxes'
description: 'Control for BUG-KHQXHH. The editing surface wears .md-body and inherits the shared markdown stylesheet, whose task-list rules key on input[type=''checkbox''] - an element Milkdown''s GFM preset never emits (it puts the state in a data-checked attribute instead). That mismatch made task lists render as indistinguishable bullets while the serializer corpus gate stayed green, because the markdown round-tripped perfectly the whole time. Two layers. The unit suite (16 tests) mounts the real editor and asserts the DOM shape, the click-toggle, undo, the toolbar command and the three-state pressed logic. The e2e pair asserts what jsdom cannot decide: that the shared sheet''s :has() rule actually suppressed the bullet (computed list-style-type), and that the label sits on the checkbox''s row with no stray margin. Both assert on the same input[type=''checkbox''] selector the stylesheet uses, which is what couples the editor to the read-only marked render.'
kind: test
location: frontend/src/components/forms/milkdown/taskList.test.ts + e2e/tests/checkboxes.spec.ts
status: active
---

## Why this measure

`why5` on BUG-KHQXHH: rendering parity between the editor (ProseMirror) and the
read-only view (marked) is an assumed property with no test asserting it. The
two surfaces use different libraries but share one stylesheet, and the coupling
is held only by a prose comment in `markdown-content.css`.

This measure converts that assumption into a check. It asserts the editor emits
the same `input[type='checkbox']` shape the shared CSS selects on, so a future
construct that renders differently in one surface fails a test instead of
shipping invisible.

## Non-vacuity

Verified, and the verification itself was instructive.

- **Unit suite**: 12 of 15 tests failed against the pre-fix editor. The 3 that
passed were the negative cases (plain bullets render no checkbox, task lists
round-trip unchanged), which is correct — the bug never touched serialization.
- **E2E pair**: both fail with the node view disabled.

The e2e check needs `go build -o bin/rela-server` after `npm run build:e2e`,
because the server embeds the SPA bundle at compile time. A first attempt
skipped that and the tests passed against a stale binary, which briefly looked
like the CSS rules were unnecessary. They are not: removing `display:
inline-block` puts the label under the checkbox (`content.x` equals
`checkbox.x`), and removing the paragraph rule restores a 14px bottom margin
under every row. Each rule now has an assertion that fails without it.
