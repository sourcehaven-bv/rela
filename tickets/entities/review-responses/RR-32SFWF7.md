---
id: RR-32SFWF7
type: review-response
title: Explicit zero ts ignored
finding: A PATCH with ts 0001-01-01 is treated as keep the ts.
severity: nit
reason: A zero time is never a meaningful booking time; treating it as absent matches the Go zero value.
status: wont-fix
---
