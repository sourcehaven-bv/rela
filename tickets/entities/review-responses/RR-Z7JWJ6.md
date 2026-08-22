---
id: RR-Z7JWJ6
type: review-response
title: GC deleted data even when the pre-delete version capture failed
finding: The capturer logged capture errors and the drop steps deleted anyway. On the unattended GC sweep a transient WriteVersion failure (pool exhaustion, timeout) meant hard-deleting entities with no entity_versions row, no live row, and (unlike fs) no git — the one path that destroys data with no human in the loop lost its only recovery artifact silently.
severity: significant
resolution: 'Capture failures are now HARD errors on every destructive path (commit bddc13f3): capturer methods return errors, captureEntityDelete/captureRelationDelete propagate, and drop_entities/drop_relations/rename_relation_type abort BEFORE the delete. The operator (or next sweep tick) retries once versioning is healthy — steps are idempotent. Cascade-deleted relations (already gone when captured) remain best-effort with the error surfaced in step notes. Pinned by TestDropEntities_AbortsWhenCaptureFails (failing capture → run errors, entity survives).'
status: addressed
---
