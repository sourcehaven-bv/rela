---
id: RR-EUX4BX
type: review-response
title: 'Staged files are invisible to dirty tracking: navigate-away silently discards them with no warning'
finding: |-
    The plan deliberately keeps staged files OUT of `formData` (correct, to avoid POSTing a File as the property value). But it does not account for the consequence: `checkDirty()` (`DynamicForm.vue:1380-1388`) computes dirtiness ONLY from `JSON.stringify({formData, relations, content})` plus `pendingCardChanges`. A staged file changes none of these.

    So a user who fills nothing but attaches a file has `dirty === false`. Both unsaved-changes guards then let them leave silently: the in-app router guard (`DynamicForm.vue:1596` `if (!dirty.value) return true`) and the native beforeunload handler (`handleBeforeUnload`, ~line 1397, `if (dirty.value)`). The staged file is discarded with no prompt.

    This is silent data loss of user-selected input — precisely the failure class the ticket says it must not ship ('Silent failure is the specific thing this ticket must not ship'). It is also the more likely everyday scenario than the post-create upload failure the plan does carefully handle.

    Fix: include staged-file state in the dirty computation. Cheapest correct form is to fold a stable projection (e.g. per-property array of `name:size`) into checkDirty's serialized payload, or set a separate `hasStagedFiles` flag ORed into `dirty.value` alongside `hasCardChanges` — which is the existing precedent for exactly this situation (a non-formData source of dirtiness). Add an edge case + test.
severity: significant
resolution: 'Added section 3b to the plan''s Technical Approach. Staged files are folded into checkDirty() via a `hasStagedFiles` flag ORed alongside the existing `hasCardChanges` — the established precedent for a dirtiness source outside formData. Added a test-plan row: stage a file, assert isDirty() is true and the navigation guard prompts. Also interacts with RR-6ZAOMK''s pinned ordering, where dirty.value = false now fires only when every upload succeeded.'
status: addressed
---

## Finding

The plan's central design decision — keep staged files out of `formData` — is
right, but it has an unexamined consequence.

`checkDirty()` (`DynamicForm.vue:1380-1388`):

```js
const currentData = JSON.stringify({
  formData: formData.value,
  relations: relations.value,
  content: content.value,
})
const hasCardChanges = pendingCardChanges.value.size > 0
dirty.value = currentData !== originalData.value || hasCardChanges
```

Staged files live in none of those inputs, so staging a file leaves `dirty ===
false`.

Both exit guards consult exactly that flag:

- in-app router guard — `DynamicForm.vue:1596`: `if (!dirty.value) return true`
- native `beforeunload` — `handleBeforeUnload` (~1397): `if (dirty.value)`

A user who opens a create form, attaches a file, and navigates away loses it
with **no prompt at all**.

## Severity rationale

This is silent loss of user-provided input. The ticket explicitly states "Silent
failure is the specific thing this ticket must not ship" — and the plan spends
real effort on the *post-create upload failure* path while missing this one,
which is considerably more likely in ordinary use (attaching a file and then
getting distracted is routine; a 413 is not).

## Resolution

`pendingCardChanges` is the existing precedent for a dirtiness source outside
`formData` — it's ORed in as `hasCardChanges`. Do the same:

```js
const hasStagedFiles = Object.values(stagedFiles.value).some((a) => a.length > 0)
dirty.value = currentData !== originalData.value || hasCardChanges || hasStagedFiles
```

Add to Edge Cases, and add a test: stage a file, assert `isDirty()` is true and
the navigation guard prompts.
