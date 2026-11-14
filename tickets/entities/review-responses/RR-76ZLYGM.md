---
id: RR-76ZLYGM
type: review-response
title: Owner delete removes one side of ledger transactions
finding: Deleting an account drops its postings, leaving the other accounts of those transactions out of balance.
severity: significant
reason: Documented in GUIDE-records (archive an account with postings instead). Refusing the delete needs a pre-delete hook in entitymanager; tracked in TKT-EV8U8T4.
status: deferred
---
