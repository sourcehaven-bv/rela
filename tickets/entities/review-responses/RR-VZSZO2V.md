---
id: RR-VZSZO2V
type: review-response
title: render loses arithmetic, concat and unary minus
finding: render printed ... for these nodes.
severity: nit
reason: 'These nodes cannot occur in a condition program: the compiler rejects arithmetic, concatenation and unary minus on a non-literal. The render cases were removed.'
status: wont-fix
---
