---
id: RR-Z1LUO5
type: review-response
title: Stale retention across entity switch / forced refetch; single-slot lastEdit reusable by an unrelated revert
finding: |-
    1. releaseAll() existed on the composable but was NEVER called. DynamicForm has no :key per entity, so switching entities without a remount — or any loadEntity(true) refetch — left retained values from the previous form state in place. A subsequent reveal could restore one entity's value onto another, or resurrect a value the server no longer has.

    2. lastEdit was never cleared. A hide NOT caused by a tracked user edit (a programmatic change, a reload) would revert using a stale snapshot from an unrelated earlier edit.
severity: significant
resolution: |-
    1. loadEntity now calls hiddenPolicy.releaseAll() immediately before replacing formData. Lossless: the stored value is safe on the server, so a later reveal reads it back from properties.

    2. captureTriggerSnapshot() now CONSUMES lastEdit (sets it to null), so a snapshot is good for exactly one hide resolution. An empty snapshot means the hide was not user-initiated, and the revert leaves the form alone rather than restoring a stale value.

    Note a reviewer confirmed the original was less broken than feared for the concurrent-edit case: captureTriggerSnapshot is called BEFORE the await, so a later updateField during an open dialog could not poison an already-captured snapshot. The fix matters for the untracked-hide case.
status: addressed
---
