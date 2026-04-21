---
id: RR-DLP50
type: review-response
title: cleanupTempFiles swallows Walk errors silently
finding: Walk errors during temp-file cleanup (e.g., permission denied) silently prevented cleanup with no diagnostic. Operator debugging 'why does my .tmp file not go away on restart' had zero breadcrumbs.
severity: minor
resolution: 'Added slog.Warn(''fsstore: temp-file cleanup walk failed'', ...) when Walk returns an error. Doesn''t change control flow (cleanup is best-effort), just adds the breadcrumb.'
status: addressed
---
