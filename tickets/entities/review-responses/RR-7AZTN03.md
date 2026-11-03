---
id: RR-7AZTN03
type: review-response
title: loadComments does not order two loads for one row
finding: An older thread for the same row can land after a newer one that includes a just-added comment.
severity: nit
reason: Same row, so no cross-entity leak; the next comment action reloads. Separate from the switch bug.
status: deferred
---
