---
id: RR-4ELY7CO
type: review-response
title: Relation restore under local stalls when an end is gone
finding: A relation the remote deleted with an end entity could not be created again, yet was reported restored; a 409 on restore looped.
severity: significant
resolution: keepEdited holds the relation when an end is not linked or is gone this run; Resolve refuses take local then; a 409 on a restore links the row again. TestDeletes_RemoteDeletedRelation.
status: addressed
---
