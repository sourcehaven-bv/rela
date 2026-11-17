---
id: RR-9HY2CEG
type: review-response
title: Clock read twice
finding: updated_at and the audit time come from two clock reads.
severity: nit
reason: The audit time is when the line is written; a minute's skew in tests is harmless and the values serve different purposes.
status: wont-fix
---
