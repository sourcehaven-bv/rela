---
id: RR-1LMJSQ
type: review-response
title: AC-7 cannot be tested in DynamicForm.guard.test.ts; dirty re-seed has a real hole
finding: 'DynamicForm.guard.test.ts does not mount DynamicForm — makeFormHarness (:19-36) is a hand-copied replica of the guard, so extending it proves nothing about the reset. AC-7 must move to a test that mounts the component and reads defineExpose''s isDirty(). Substantively: adoptLockedFieldValues (:749) mutates formData after originalData is re-seeded, so on a form with an entry-locked field record 2 is dirty the moment the dry-run resolves — a spurious unsaved-changes prompt after every record.'
severity: significant
status: addressed
resolution: >-
  AC-7 moved to a test that mounts the component and reads defineExpose's isDirty(). New plan step 8 re-seeds originalData after the awaited refreshStagedAffordances so adoptLockedFieldValues cannot leave record 2 spuriously dirty.
---
