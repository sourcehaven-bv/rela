---
id: RR-KF9C87
type: review-response
title: Export reads res.Entity before the error check
finding: entity := res.Entity ran before the err and found checks.
severity: nit
resolution: Moved after both checks.
status: addressed
---
