---
id: RR-7FYTONC
type: review-response
title: Rows can be orphaned when the delete hook fails
finding: The alias hook drops rows after the entity delete commits; if DropOwner fails, a later entity reusing the id inherits them.
severity: significant
resolution: Database triggers on the entity tables drop an owner's rows when its last face is gone and move them on rename, inside the entity write's transaction (pg 0049, sqlite v40). A failed drop rolls the delete back. TestRecords_RowsFollowOwnerInsideTheEntityWrite.
reason: 'Holds only for the file and memory backends: there the rows still go in a post-commit hook whose error is only logged, and a single local user is the only writer.'
status: addressed
---
