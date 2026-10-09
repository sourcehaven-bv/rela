---
id: RR-QCO6SP
type: review-response
title: Move no longer cross-checks what DeleteFace removed
finding: The carried set and the deleted set came from two separate queries, so an edge written between them would be deleted without being carried.
severity: significant
resolution: The move deletes the carried originals itself and then fails if DeleteFace removed any edge, which rolls back on pg and sqlite.
status: addressed
---
