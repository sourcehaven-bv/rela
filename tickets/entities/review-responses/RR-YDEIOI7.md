---
id: RR-YDEIOI7
type: review-response
title: CI step placement and flow validate warnings
finding: The new step runs after rela validate, and flow validate exits 0 on warnings.
severity: nit
reason: Order does not matter for two independent checks; warnings are advisory by design of flow validate.
status: wont-fix
---
