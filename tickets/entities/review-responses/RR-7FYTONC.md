---
id: RR-7FYTONC
type: review-response
title: Rows can be orphaned when the delete hook fails
finding: The alias hook drops rows after the entity delete commits; if DropOwner fails, a later entity reusing the id inherits them.
severity: significant
resolution: Database triggers on the entity tables drop an owner's rows when its last face is gone and move them on rename, inside the entity write's transaction (pg 0046, sqlite v38). A failed drop rolls the delete back. TestRecords_RowsFollowOwnerInsideTheEntityWrite.
reason: The hook error is logged by the alias fanout, and the window is a failed delete followed by reuse of the same id. Deleting inside the store transaction, or a tombstone, is tracked in TKT-EV8U8T4.
status: addressed
---
