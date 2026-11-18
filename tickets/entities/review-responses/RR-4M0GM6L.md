---
id: RR-4M0GM6L
type: review-response
title: Write callbacks used the outer ctx
finding: writeChecked callbacks ignored the transaction ctx, so nested writes could open a second transaction.
severity: significant
resolution: Callbacks now take (txCtx, view); joinedReplace.after passes txCtx on.
status: addressed
---

writeChecked callbacks ignored the transaction ctx, so nested writes could open a second transaction.
