---
id: RR-BLECTJJ
type: review-response
title: Deleting a subject or output is blocked by managed edges
finding: Cascade relation authorization checks the edge's source type (_flow-run), which only the engine may write, so a user cannot delete a subject or an output with a cascade.
severity: significant
resolution: 'Plan changed: an edge of a managed relation removed because its far endpoint is deleted is not authorized as a relation write of the deleting principal; the delete of the endpoint is the authorized act. The guard covers direct managed writes only. Rename keeps edges by id. A conversion keeps a managed edge when the target type may be its endpoint and drops it otherwise (code review, TKT-BC5G8RP).'
status: addressed
---
