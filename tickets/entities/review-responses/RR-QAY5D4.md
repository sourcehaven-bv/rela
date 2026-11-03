---
id: RR-QAY5D4
type: review-response
title: SPA ignores can_sync and has no settings can_edit hint
finding: The SPA does not read can_sync and always shows the sync button; GET /_settings exposes no can_edit, and saveSettings drops the 403 detail. (cranky-code-reviewer)
severity: minor
resolution: Deferred to TKT-FLTR9P.
reason: The server check is authoritative; this is a UX follow-up in the frontend, tracked as TKT-FLTR9P.
status: deferred
---
