---
id: RR-NWJTCDP
type: review-response
title: Guards in the Tx must use the new entity
finding: Engine reads through Subjects see the pre-write row inside the transaction.
severity: minor
resolution: OnChange takes the post-write entity; the in-transaction engine copy has a Subjects that errors.
status: addressed
---
