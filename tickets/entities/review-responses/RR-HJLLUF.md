---
id: RR-HJLLUF
type: review-response
title: Double-submit via Cmd+Enter creates two entities and two sets of uploads
finding: |-
    `handleSubmit` (DynamicForm.vue:986) has no re-entrancy guard. `PendingButton` guards its own click, but `handleKeydown` (DynamicForm.vue:1511) calls `handleSubmit()` DIRECTLY, bypassing it. Two Cmd+Enter presses during an in-flight submit produce create=2 and uploadAttachment=2 — two entities, both with the file attached.

    The hole is pre-existing, but this commit materially widens it. Before, `saving` spanned one POST. Now it spans that POST plus N sequential multipart uploads, so a large file on a slow link holds the window open for tens of seconds. It is also newly worse in kind: previously a duplicate entity, now a duplicate entity AND duplicate blobs.

    Fix: `if (saving.value) return` at the top of handleSubmit, after the formConfig guard. On the operation, not in handleKeydown — otherwise the next caller reintroduces it.
severity: critical
resolution: 'Added `if (saving.value) return` at the top of handleSubmit (on the operation, not in handleKeydown). Also added a `createdEntityId` guard so a COMPLETED create cannot be re-submitted from the same mounted form — `saving` is reset in `finally`, so it only covers an in-flight submit, and the failure path stays on a live form. Pinned by two tests: ''ignores a second submit while one is in flight'' (create=1, uploads=1) and ''a second submit after a failure does not create a second entity''.'
status: addressed
---

## Finding

Verified: `handleSubmit` (`DynamicForm.vue:986`) guards only `!formConfig.value`
and `isEdit`. There is no `saving` check.

`handleKeydown` (`DynamicForm.vue:1511`) calls `handleSubmit()` directly, so the
`PendingButton` in-flight guard is bypassed entirely on the keyboard path.

## Why this is critical now

The race is pre-existing, but its window was one POST. This commit appends N
sequential multipart uploads inside the same window — for a large file that is
tens of seconds rather than milliseconds. A theoretical race becomes one that is
straightforward to hit.

The consequence also changed in kind: previously a duplicate entity; now a
duplicate entity plus duplicate attachment blobs.

## Fix

```js
async function handleSubmit() {
  if (!formConfig.value) return
  if (saving.value) return   // ← re-entrancy guard
  ...
```

Guard the operation, not the caller — fixing it inside `handleKeydown` leaves
the next caller free to reintroduce it.
