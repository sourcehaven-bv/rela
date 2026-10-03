---
id: RR-3DPREG
type: review-response
title: Loop body is not folded itself
finding: fold stored the raw loop body, so A A B repeated drew A twice per iteration.
severity: minor
resolution: The body goes through fold too; TestFoldNestedBody.
status: addressed
---
