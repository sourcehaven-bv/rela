---
id: RR-5F3G5E
type: review-response
title: Other app-log sites still log raw paths
finding: api_v1.go, router.go, nextaction_handler.go and jwtgate.go error lines log r.URL.Path; the guide implied the app log was clean.
severity: significant
reason: 'Out of #1782''s scope (the access log). These are error-only lines where the id aids diagnosis, a per-site trade-off. The guide now says other app-log error lines can contain the full path. Filed as a follow-up ticket to decide per site.'
status: deferred
---
