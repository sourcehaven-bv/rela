---
id: RR-Z3BLF6H
type: review-response
title: 'Design: chain hash covered only doc'
finding: The row hash covered prev_hash, seq and doc. The at, op and principal_user columns could be edited without a break; editing at moves a row in or out of a prune cutoff, and the denormalized op and user columns would mislead SQL readers.
severity: significant
resolution: ChainHash now takes a Row with every stored column (seq, at, op, principal_user, doc) and length-prefixes the strings. testColumnEditsBreak edits each column and expects a break.
status: addressed
---
