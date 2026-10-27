---
id: RR-NO1I2K
type: review-response
title: pg generated sha256 column may recompute on rename
finding: A stored generated column might be recomputed on UPDATEs that do not touch bytes.
severity: nit
reason: Since PostgreSQL 13 a stored generated column is recomputed only when a column it depends on is updated, so the entity_id rename does not rehash. CI and the documented deployments run PostgreSQL 16.
status: wont-fix
---
