---
id: RR-P0U4VQP
type: review-response
title: cancel does not check the header type
finding: cancel read GetAddress without checking that the entity is a _flow-run.
severity: nit
resolution: cancel uses the header helper, which answers not found for another type.
status: addressed
---
