---
id: RR-Y7Q6D
type: review-response
title: UpdateEntity fires for relations-only PATCH
finding: PATCH with only a relations payload still calls a.entityManager.UpdateEntity, which writes the entity file, re-runs validation, bumps mtime, and emits a broker entity-updated SSE event despite no entity bytes changing. Skip the UpdateEntity call when req.Properties == nil && req.Content == nil.
severity: critical
resolution: handleV1UpdateEntity now gates the UpdateEntity call on `req.Properties != nil || req.Content != nil`. Relations-only PATCH skips the entity rewrite, mtime bump, validation re-run, and entity-updated SSE event.
status: addressed
---
