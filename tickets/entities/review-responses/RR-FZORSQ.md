---
id: RR-FZORSQ
type: review-response
title: Rename leaves the previous editor on re-keyed rows
finding: Entity rename re-keys rows without touching last_edited_by_*. If the best-effort synchronous rename capture is missed, a later sweep could record an update credited to the previous content editor.
severity: minor
reason: 'Pre-existing on pgstore and a documented decision: docs/postgres-backend.md states re-keyed relations keep their last content editor. Only reachable when the synchronous rename capture fails. Changing rename semantics is out of scope for this bug.'
status: deferred
---
