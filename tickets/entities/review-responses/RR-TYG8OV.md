---
id: RR-TYG8OV
type: review-response
title: Store godoc states the empty case but not the populated one
finding: Callers need the iff direction and the current wording is both vague and inconsistent with the attachment path
severity: minor
resolution: 'Godoc restated in the direction callers need: DeletedEntities is populated if and only if the entity file was removed.'
status: addressed
---

The godoc says *"DeletedEntities is empty whenever the entity itself survived"*
— a statement about when it is empty, which tells a caller nothing about when it
is populated. Combined with the attachment-path bug it is also currently wrong.

State it in the direction callers need: `DeletedEntities` is populated **if and
only if** the entity file was removed.
