---
id: RR-LWB7M
type: review-response
title: validTopLevelKeys must include the new key
finding: internal/dataentryconfig/validate.go:16 has validTopLevelKeys map; missing entries cause 'unknown key' rejection. Plan doesn't mention updating it. Easy to miss, easy to break the build.
severity: minor
resolution: Plan explicitly adds entity_views to validTopLevelKeys in internal/dataentryconfig/validate.go:16.
status: addressed
---
