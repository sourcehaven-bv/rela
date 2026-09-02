---
id: RR-6ZAOMK
type: review-response
title: Plan doesn't specify upload placement relative to dirty-reset and the 'Entity created successfully' toast
finding: |-
    The plan says to upload 'after `entitiesStore.create` (line 1014) and before the navigation block (line 1050)', but that span contains three ordering-sensitive statements it never assigns a position relative to:

    1. `dirty.value = false` (line 1037)
    2. `if (props.embedded) { emit('inline-created', entity); return }` (lines 1043-1046)
    3. `uiStore.success('Entity created successfully')` (line 1048)

    Each placement is a real decision:

    - If uploads run AFTER `uiStore.success(...)`, the user sees a green 'Entity created successfully' toast immediately followed by an upload error toast — contradictory messaging about one action. If BEFORE, the success toast can be made accurate (or suppressed/reworded on partial failure).
    - If uploads run after `dirty.value = false`, then a failed upload leaves the form non-dirty with staged files still pending — interacting badly with RR-EUX4BX's fix and letting the user leave without a prompt after a failure.
    - The plan does flag the embedded early-return (good), but only as 'run before it', without resolving how it composes with the toast and dirty-reset above it.

    Fix: specify the exact statement order in the plan so implementation is mechanical, as the checklist requires ('no design decisions left'). Suggested: uploads immediately after `create` returns and after `surfaceWarnings`, BEFORE dirty-reset/embedded-return/success-toast; on failure, replace the success toast with a partial-success message naming what failed.
severity: significant
resolution: 'The plan''s Technical Approach now pins the exact statement order: (1) create + surfaceWarnings, (2) upload staged files continue-on-error, (3) dirty.value = false only if every upload succeeded, (4) embedded early-return, (5) outcome-dependent toast. Uploads precede the success toast so a failure cannot produce a green ''Entity created successfully'' followed by a red error describing the same action. Added a test-plan row asserting a failed upload leaves the form dirty and emits no bare success toast.'
status: addressed
---

## Finding

The plan's insertion point is "after line 1014, before line 1050". That span is
not inert — it contains three statements whose order relative to the uploads
changes user-visible behaviour, and the plan assigns none of them:

```js
dirty.value = false                              // 1037

if (props.embedded) {                            // 1043
  emit('inline-created', entity)
  return                                          // 1045
}

uiStore.success('Entity created successfully')   // 1048
```

**Toast contradiction.** Uploading after line 1048 means a failed attachment
produces a green "Entity created successfully" immediately followed by a red
upload error. Two toasts describing one user action, disagreeing.

**Dirty-reset interaction.** Uploading after line 1037 means a failure leaves
`dirty === false` while staged files remain unconsumed. Combined with
RR-EUX4BX's fix, that is contradictory: staged files are supposed to make the
form dirty, but the reset already fired.

**Embedded composition.** The plan flags the early return but doesn't say how it
composes with the two statements above it.

## Why significant, not minor

The checklist standard is that implementation should be "mechanical — no design
decisions left". Three unresolved ordering decisions in the single function
being modified is exactly the gap that gets resolved arbitrarily under
implementation pressure, and the arbitrary choice here produces contradictory
UX.

## Suggested resolution

Pin the order explicitly in the plan:

1. `create` → `surfaceWarnings`
2. **upload staged files** (collect failures)
3. `dirty.value = false` only if all uploads succeeded (else keep dirty so the
guard protects the un-uploaded files)
4. embedded early-return
5. success toast — worded per outcome: full success vs. "Entity created, but the
attachment failed: <detail>"
