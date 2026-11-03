---
id: RR-IPO2WZ0
type: review-response
title: A host reload strands the hold
finding: DynamicForm.loadEntity(true) (after a transition save, an attachment change or a world switch) replaced formData and content wholesale, so a held field showed the server value under a stale error while still held.
severity: significant
resolution: useAutoSave exposes unsavedFields(); DynamicForm.loadEntity keeps those values on screen, and EntityDetail.applyView keeps a held body when the same row is read again. Tests in DynamicForm.entitySwitch.test.ts and EntityDetail.bodyConflict.test.ts fail without the change.
status: addressed
---
