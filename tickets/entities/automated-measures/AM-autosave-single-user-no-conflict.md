---
id: AM-autosave-single-user-no-conflict
type: automated-measure
title: 'Property test: one user never sees an autosave conflict'
description: Random edits, saves, live echoes and answer orders against the fake API; a single client must never report a conflict and the held server state is never older than its last save answer.
kind: test
location: frontend/src/composables/useEntitySession.prop.test.ts and EntityDetail.bodyConflict.test.ts
status: proposed
---
