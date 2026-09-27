---
id: RR-31AL2A
type: review-response
title: Face refusal broader than needed
finding: Refusing on any rq.Faces could allow the case where the default face is among the granted faces.
severity: minor
reason: Fail-closed and correct. Narrowing it needs a proof that a default-face grant covers exactly the default-state tail semantics per backend; not needed for the feature.
status: deferred
---
