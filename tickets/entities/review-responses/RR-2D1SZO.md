---
id: RR-2D1SZO
type: review-response
title: Base-less DELETE was a blind delete, contradicting the docstring
finding: handleSyncDelete guarded with `if ifMatch != "" && ifMatch != cur`, so a DELETE with NO If-Match on an existing record passed and deleted it unconditionally — a blind delete. The docstring claimed 'a client only deletes what it last saw'. Code and comment disagreed, and the unsafe one was the code. Asymmetric with PUT, which requires a declared base.
severity: significant
resolution: Added deletePreconditionOK requiring a non-empty If-Match that matches the current hash. A base-less delete of an existing record is now 412, never a blind delete (force-delete is the client's --force path, which re-reads the hash). Symmetric with push. Regression added to TestSync_DeleteEntity asserts a no-If-Match delete returns 412 and the record survives.
status: addressed
---
