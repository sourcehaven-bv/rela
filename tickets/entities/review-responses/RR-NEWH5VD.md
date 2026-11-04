---
id: RR-NEWH5VD
type: review-response
title: Deleting a person leaves them as the actor of pile items
finding: After a person entity is deleted, their id stays as the actor on items in other users' piles. rela accepts an explicit id on create, so a later person with the same id would be named as the sender of old notifications.
severity: minor
resolution: DeleteOwner now clears the owner as actor on every item in both kvpiles and pgpiles; the pilestest conformance case 'delete owner clears the actor on every pile' pins it on both backends (pg run against a real database).
status: addressed
---
