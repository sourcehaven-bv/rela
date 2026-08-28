---
id: RR-XABO9R
type: review-response
title: Dirty-tracking via bubbling DOM events missed every non-native widget
finding: InlineCreateFormModal tracked unsaved input with @input/@change on a wrapper div. Vue custom events do not bubble as native DOM events, so this missed a relation-picker selection (Vue `update`), a markdown body edit (CodeMirror emits no bubbling native input), and a wizard step change (a button click). A user who wrote a body, picked a relation and hit Escape lost the work with NO confirm — the exact failure the code's own comment claimed to guard against.
severity: significant
resolution: 'Replaced the heuristic with the form''s real state: DynamicForm defineExposes isDirty()/isSaving(), and the modal asks it. Also added an isSaving() guard so Escape cannot tear the form out mid-POST. Pinned by InlineCreateFormModal.test.ts; mutation-checked (stubbing isDirty to false fails the discard tests).'
status: addressed
---
