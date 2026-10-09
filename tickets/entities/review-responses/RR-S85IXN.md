---
id: RR-S85IXN
type: review-response
title: 'Design: minor items'
finding: Retry classification, payload contents, docs gaps, pool starvation, unknown keys.
severity: minor
resolution: Config errors end the job without retry; payload holds only automation, file, id, face, hops; docs cover stale entity, nil old_entity, idempotent scripts, in-memory loss. Pool starvation and strict action keys noted as risks/deferred.
status: addressed
---
