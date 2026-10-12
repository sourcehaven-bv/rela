---
id: RR-YHUZ7B
type: review-response
title: Whole-entity delete authorizes hidden faces
finding: DeleteEntity and SoftDeleteEntity pass when one face is readable and then authorize every face; a hidden face changes the answer
severity: significant
resolution: Deferred to TKT-Z47F27; the guide states the exception
reason: 'Rename has the same rule today and fixing it means deciding per readable face as DeleteEntityFace does: delete readable faces and keep the entity with its hidden faces and edges. That changes delete semantics for faced types and needs its own design and tests. This change removes the larger gap (rows with no readable face at all).'
status: deferred
---
