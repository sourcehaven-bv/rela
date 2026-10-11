---
id: RR-4L8CVH
type: review-response
title: 500s logged at Warn without request context
finding: Other 500 sites use slog.Error.
severity: minor
resolution: logInternalError uses slog.ErrorContext(r.Context(), ...).
status: addressed
---
