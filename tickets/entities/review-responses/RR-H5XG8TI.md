---
id: RR-H5XG8TI
type: review-response
title: EvalErrors never filled for store-answered conditions
finding: A lowered rule never adds to EvalErrors, while the same condition judged in Go can. This was undocumented.
severity: nit
resolution: Documented on Result.EvalErrors.
status: addressed
---
