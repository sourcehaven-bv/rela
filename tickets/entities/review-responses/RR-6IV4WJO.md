---
id: RR-6IV4WJO
type: review-response
title: Delete conflicts of a locally deleted row are hidden on the web
finding: The web conflict list gates each entry by the reader's access to the local row. A local_deleted entry names a row that no longer exists, so the web hides it; only the CLI lists and resolves it.
severity: minor
reason: The per-row gate must not be loosened for a row that is gone. Documented in the guide; the CLI covers the operator. A follow-up can gate these entries by remote:sync plus a type-level read check.
status: deferred
---
